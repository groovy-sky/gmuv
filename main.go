package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/imroc/req/v3"
	"github.com/urfave/cli/v2"
)

var (
	execPath string
)

const (
	repoMdStruct = `
## [{{.Repository.Name}}]({{.Repository.HTMLURL}})`
	repoCliStruct = `
## [{{.Repository.Name}}]({{.Repository.HTMLURL}})`
	repoErrStruct  = ` - {{.State}}`
	fileHeadStruct = `
* {{.Repository.HTMLURL}}/blob/{{.Repository.DefaultBranch}}/`
	fileStruct = `{{.Path}}

| URL | State |
| --- | --- |
`
	linkMdStruct = `| {{.Link}} | {{.State}} |
`
	linkCliStruct = `| {{.Link}} | {{.State}} |
`
)

type Repository struct {
	// Part of Github API response strutures
	// https://github.com/google/go-github/blob/2d872b40760dcf7080786ece0a4735509ff071f4/github/repos.go#L28
	Name          *string `json:"name,omitempty"`
	URL           *string `json:"url,omitempty"`
	Fork          *bool   `json:"fork,omitempty"`
	Disabled      *bool   `json:"disabled,omitempty"`
	Archived      *bool   `json:"archived,omitempty"`
	CloneURL      *string `json:"clone_url,omitempty"`
	HTMLURL       *string `json:"html_url,omitempty"`
	DefaultBranch *string `json:"default_branch,omitempty"`
	Size          *int    `json:"size,omitempty"`
	// Custom fields
	WebUrl *string // for relative paths check
}

// Checked URL structure
type MdLink struct {
	Link    *string
	State   *int
	Succeed *bool
}

// Checked MD file matched URL and path to the file
type MdFile struct {
	Path     *string
	LinkList *[]MdLink
}

// Generated reports structure
type MdReport struct {
	Repository *Repository
	MdFileList *[]MdFile
	ZipUrl     *string
	ZipName    *string
	ZipPath    *string
	State      *string
	AllLinksOK *bool
}

type MdReportList struct {
	Mu      sync.Mutex
	Reports []*MdReport
}

// Writes results in specified format
func generateReport(md *MdReport, out *os.File) {
	var linkStruct, repoStruct string
	outInfo, _ := out.Stat()
	if outInfo.Name() != "stdout" && getFileExtension(outInfo.Name()) == "md" {
		linkStruct = linkMdStruct
		repoStruct = repoMdStruct
	} else {
		linkStruct = linkCliStruct
		repoStruct = repoCliStruct
	}
	t := template.Must(template.New("repo").Parse(repoStruct))
	t.Execute(out, md)
	if md.State != nil {
		t = template.Must(template.New("repoErrStruct").Parse(repoErrStruct))
		t.Execute(out, md)
	} else if len(*md.MdFileList) != 0 {
		for _, file := range *md.MdFileList {
			t = template.Must(template.New("fileHead").Parse(fileHeadStruct))
			t.Execute(out, md)
			if !*md.AllLinksOK {
				t = template.Must(template.New("file").Parse(fileStruct))
				t.Execute(out, file)
				t = template.Must(template.New("links").Parse(linkStruct))
				for _, link := range *file.LinkList {
					if !*link.Succeed {
						t.Execute(out, link)
					}
				}
			}
		}
	}
}

func getFileExtension(s string) string {
	s = strings.ToLower(s)
	ext := strings.Split(s, ".")
	return ext[len(ext)-1]
}

func checkUrl(url string, web *req.Client) (response *req.Response, ok bool) {
	response, err := web.R().Get(url)
	if err != nil {
		return response, ok
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 200:
		ok = true
	}
	return response, ok

}

// Tries to validate markdown URL
func checkMdLink(md *MdReport, l, rpath, fpath string) (result int, ok bool) {
	var webclient = req.C()
	var r *req.Response
	var url string
	// Delete last elemnt, which is a brace
	l = l[:len(l)-1]
	// Delete part containing square brackets and brace, which comes before a link
	l = l[len(regexp.MustCompile(`(^\[(.*?)]\()`).FindString(l)):]
	// Check if link starts with http/https
	url = regexp.MustCompile(`(^https?:\/\/)([\da-z\.-]+)\.([a-z\.]{2,6})\/?.*`).FindString(l)
	// Check if a domain name is resolvable and filename extension != md -> add http protocol
	// else -> add relative path to it
	if fqdn, _, _ := strings.Cut(l, "/"); !strings.Contains(l, ":") && url == "" {
		if _, err := net.LookupIP(fqdn); err == nil && getFileExtension(l) != "md" {
			url = "http://" + l
		} else {
			// Check if link starts / -> absolute path is used
			// if not -> relative path should be used
			if l != "" && string(l[0]) == "/" {
				url = *md.Repository.WebUrl + l
			} else {
				url = *md.Repository.WebUrl + rpath + l
			}
		}
	}
	// Test URL if link is not an e-mail address
	if strings.HasPrefix(l, "mailto:") {
		ok = true
	} else {
		r, ok = checkUrl(url, webclient)
	}

	// Store HTTP response if there is one
	if r != nil && r.Err == nil {
		result = r.StatusCode
	}
	return result, ok
}

// Searches for *.md files and loads its content from *.zip archive
func findAndCheckMdFile(md *MdReport, f *zip.File) {
	_, fileFullPath, _ := strings.Cut(f.FileHeader.Name, "/")
	fileRelativePath, _, _ := strings.Cut(fileFullPath, f.FileInfo().Name())

	if fileRelativePath != "" {
		fileRelativePath = "/" + fileRelativePath + "/"
	} else {
		fileRelativePath = "/"
	}
	if !f.FileInfo().IsDir() {
		fileName := f.FileInfo().Name()
		ext := getFileExtension(fileName)
		// Proceed if file is not a directory and has .md extension
		if strings.ToLower(ext) == "md" {
			links := []MdLink{}
			zipContent, err := f.Open()
			if err != nil {
				state := (*md.State + " [ERR] Couldn't open " + f.FileInfo().Name() + " file: \n\t" + err.Error())
				md.State = &state
				return
			}
			defer zipContent.Close()

			content, err := ioutil.ReadAll(zipContent)
			if err != nil {
				state := (*md.State + " [ERR] Couldn't load " + f.FileInfo().Name() + ": \n\t" + err.Error())
				md.State = &state
				return
			}
			// Use regexp for matching Markdown URL
			matches := regexp.MustCompile(`\[[^\[\]]*?\]\(.*?\)|^\[*?\]\(.*?\)`).FindAll(content, -1)
			for _, val := range matches {
				url := string(val)
				state, ok := checkMdLink(md, url, fileRelativePath, fileFullPath)
				if !ok {
					*md.AllLinksOK = false
					mdLinkVal := MdLink{&url, &state, &ok}
					links = append(links, mdLinkVal)
				}
			}
			if len(links) > 0 {
				if md.MdFileList == nil {
					file := []MdFile{{&fileFullPath, &links}}
					md.MdFileList = &file
				} else {
					file := MdFile{&fileFullPath, &links}
					*md.MdFileList = append(*md.MdFileList, file)
				}
			}
		}
	}
}

// Reads files from *.zip archive and filters *.md. At the end deletes folder with downloaded archive
func checkMdFiles(md *MdReport, Mu *sync.Mutex, out *os.File) {
	//defer os.RemoveAll(*md.ZipPath)
	reader, err := zip.OpenReader(filepath.Join(*md.ZipPath, *md.ZipName))
	if err != nil {
		*md.State = ("[ERR] Couldn't open archive " + *md.ZipName + ".\n\t" + err.Error())
		return
	}
	defer reader.Close()

	for _, f := range reader.File {
		findAndCheckMdFile(md, f)
	}
	if md.MdFileList == nil {
		s := "[INF] No markdown links were found."
		md.State = &s
	} else if *md.AllLinksOK {
		s := "[INF] No inactive/broken links were found."
		md.State = &s
	}
	Mu.Lock()
	defer Mu.Unlock()
	generateReport(md, out)
}

// Downloads and stores Github repository as zip archive
func downloadGitArchive(md *MdReport) error {

	fullpath := filepath.Join(*md.ZipPath, *md.ZipName)
	if err := os.MkdirAll(*md.ZipPath, 0755); err != nil {
		*md.State = ("[ERR] Couldn't create " + *md.ZipPath + " path.\n\t" + err.Error())
		return err
	}

	out, err := os.Create(fullpath)
	if err != nil {
		*md.State = ("[ERR] Couldn't create " + fullpath + " file.\n\t" + err.Error())
		return err
	}
	defer out.Close()

	resp, err := http.Get(*md.ZipUrl)

	if err != nil {
		*md.State = ("[ERR] Couldn't download " + *md.ZipUrl + " file.\n\t" + err.Error())
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		*md.State = ("[ERR] Couldn't store downloaded file.\n\t" + err.Error())
		return err
	}
	return nil
}

func (l *MdReportList) Append(report MdReport) {
	l.Mu.Lock()
	defer l.Mu.Unlock()
	l.Reports = append(l.Reports, &report)
}

// Returns public/not-forked/not-archived/not-empty repository list
func GetPublicRepos(account, repo string) []*Repository {
	var resp *http.Response
	var err error
	var allRepos, outRepos []*Repository
	var singleRepo *Repository

	switch repo {
	case "":
		resp, err = http.Get("https://api.github.com/users/" + account + "/repos?type=owner&per_page=100&type=public")
		if err != nil {
			log.Fatalln(err)
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&allRepos); err != nil {
			log.Fatalln(err)
		}
		// Store only active, not forked and not empty repos
		for i := range allRepos {
			if !*allRepos[i].Fork && !*allRepos[i].Disabled && !*allRepos[i].Archived && *allRepos[i].Size > 0 {
				outRepos = append(outRepos, allRepos[i])
			}
		}

	default:
		resp, err = http.Get("https://api.github.com/repos/" + account + "/" + repo)
		if err != nil {
			log.Fatalln(err)
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&singleRepo); err != nil {
			log.Fatalln(err)
		}
		// Store response to output
		if resp.StatusCode == 200 {
			outRepos = append(outRepos, singleRepo)
		}

	}
	return outRepos

}

// Parses CLI input and starts repository check in parallel (using goroutines)
// if no specific repo was defined
func RunCLI() {
	var mdList MdReportList
	var githubAccount, githubRepo, resultOutput, reportFileName string
	var output *os.File
	var wg sync.WaitGroup

	app := &cli.App{
		Name:                 "gmuv",
		Usage:                "CLI tool to validate Markdown URLs",
		EnableBashCompletion: true,
		Action: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "username",
				Aliases:     []string{"u"},
				Value:       "",
				Usage:       "GitHub account name",
				Destination: &githubAccount,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "repository",
				Aliases:     []string{"r"},
				Value:       "",
				Usage:       "GitHub repository name",
				Destination: &githubRepo,
			},
			&cli.StringFlag{
				Name:        "output",
				Aliases:     []string{"o"},
				Value:       "file",
				Usage:       "Output format: cli or file",
				Destination: &resultOutput,
			},
			&cli.StringFlag{
				Name:        "filename",
				Aliases:     []string{"f"},
				Value:       "REPORT.md",
				Usage:       "Results filename",
				Destination: &reportFileName,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	// Do not continue if no Github account is specified
	if githubAccount == "" {
		return
	}

	path, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	execPath = filepath.Join(path, ".archives")

	switch resultOutput {
	case "cli":
		output = os.Stdout
	case "file":
		output, err = os.Create(filepath.Join(path, reportFileName))
		if err != nil {
			log.Fatalln(err)
		}
		defer output.Close()
	}

	repos := GetPublicRepos(githubAccount, githubRepo)
	reposNumber := len(repos)

	if reposNumber == 0 {
		output.Write([]byte("[INF] No repositories were found\n"))
		return
	}

	report := make([]*MdReport, reposNumber)
	mdList.Reports = report

	// Store and parse public and active repositories
	for _, repo := range repos {
		wg.Add(1)
		go func(r *Repository) {
			defer wg.Done()
			var repoUrl string
			md := new(MdReport)
			allLinksDefVal := true
			md.AllLinksOK = &allLinksDefVal
			md.Repository = r
			downloadLink := *r.HTMLURL + "/archive/refs/heads/" + *r.DefaultBranch + ".zip"
			archiveName := *r.Name + ".zip"
			downloadPath := filepath.Join(execPath, *r.Name)
			repoUrl = (*r.HTMLURL + "/blob/" + *r.DefaultBranch)
			md.ZipUrl, md.ZipName, md.ZipPath, md.Repository.WebUrl = &downloadLink, &archiveName, &downloadPath, &repoUrl
			err := downloadGitArchive(md)
			if err != nil {
				state := (*md.State + " [ERR] Couldn't download " + ": \n\t" + err.Error())
				md.State = &state
			}
			mdList.Append(*md)
		}(repo)
	}
	wg.Wait()

	mux := &mdList.Mu

	for _, md := range mdList.Reports {
		if md != nil {
			wg.Add(1)
			go func(m *MdReport) {
				defer wg.Done()
				checkMdFiles(m, mux, output)
			}(md)
		}

	}
	wg.Wait()

}

func main() {
	RunCLI()
}
