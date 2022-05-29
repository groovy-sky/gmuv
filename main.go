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
	"strconv"
	"strings"
	"text/template"
)

var execPath, githubAccount string

const (
	reportFileName = "REPORT.md"
	repoStruct     = `
## [{{.Repository.Name}}]({{.Repository.HTMLURL}})`
	repoErrStruct  = ` - {{.State}}`
	fileHeadStruct = `
* {{.Repository.HTMLURL}}/blob/{{.Repository.DefaultBranch}}/`
	fileStruct = `{{.Path}}
| Link | State |
| --- | --- |
`
	linkStruct = `| {{.Link}} | {{.State}} |
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
	// Custom fields
	WebUrl *string // for relative paths check
}

// Checked URL structure
type MdLink struct {
	Link  *string
	State *string
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
}

func generateMdReport(md MdReport, out *os.File) {
	t := template.Must(template.New("repo").Parse(repoStruct))
	t.Execute(out, md)
	if md.State != nil {
		t = template.Must(template.New("repoErrStruct").Parse(repoErrStruct))
		t.Execute(out, md)
	} else if len(*md.MdFileList) != 0 {
		for _, file := range *md.MdFileList {
			t = template.Must(template.New("fileHead").Parse(fileHeadStruct))
			t.Execute(out, md)
			t = template.Must(template.New("file").Parse(fileStruct))
			t.Execute(out, file)
			t = template.Must(template.New("links").Parse(linkStruct))
			for _, link := range *file.LinkList {
				t.Execute(out, link)
			}
		}
	}
}

func getFileExtension(s string) string {
	ext := strings.Split(s, ".")
	return ext[len(ext)-1]
}

// Tries to validate markdown URL
func checkMdLink(md *MdReport, l, rpath, fpath string) string {
	var result, url string
	// Delete last elemnt, which is a brace
	l = l[:len(l)-1]
	// Delete part containing square brackets and brace, which comes before a link
	l = l[len(regexp.MustCompile(`(^\[(.*?)]\()`).FindString(l)):]
	// Check if link starts with http/https
	url = regexp.MustCompile(`(^https?:\/\/)([\da-z\.-]+)\.([a-z\.]{2,6})\/?.*`).FindString(l)
	// Check if a domain name is resolvable and filename extension != md -> add http protocol
	// else -> add relative path to it
	if fqdn, _, _ := strings.Cut(l, "/"); !strings.Contains(l, ":") && url == "" {
		if _, err := net.LookupIP(fqdn); err == nil && strings.ToLower(getFileExtension(l)) != "md" {
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
	res, err := http.Get(url)
	if err == nil {
		defer res.Body.Close()
		if res.StatusCode > 299 {
			result = ("[ERR] " + url + " response: " + strconv.Itoa(res.StatusCode))
		} else {
			result = ("[INF] " + url + " response: " + strconv.Itoa(res.StatusCode))
		}
	} else {
		result = ("[ERR] Couldn't reach URL: " + err.Error())
	}
	return result
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
				state := checkMdLink(md, url, fileRelativePath, fileFullPath)
				mdLinkVal := MdLink{&url, &state}
				links = append(links, mdLinkVal)
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
func checkMdFiles(md *MdReport) {

	reader, err := zip.OpenReader(filepath.Join(*md.ZipPath, *md.ZipName))
	if err != nil {
		*md.State = ("[ERR] Couldn't open archive " + *md.ZipName + ".\n\t" + err.Error())
		return
	}
	defer reader.Close()

	for _, f := range reader.File {
		findAndCheckMdFile(md, f)
	}
	if err := os.RemoveAll(*md.ZipPath); err != nil {
		*md.State = ("[ERR] Couldn't cleanup " + *md.ZipName + ".\n\t" + err.Error())
		return
	}
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

func CheckGitMdLinks(r *Repository, ch chan MdReport) {
	var repoUrl string
	md := new(MdReport)
	md.Repository = r
	downloadLink := *r.HTMLURL + "/archive/refs/heads/" + *r.DefaultBranch + ".zip"
	archiveName := *r.Name + ".zip"
	downloadPath := filepath.Join(execPath, *r.Name)
	repoUrl = (*r.HTMLURL + "/blob/" + *r.DefaultBranch)
	md.ZipUrl, md.ZipName, md.ZipPath, md.Repository.WebUrl = &downloadLink, &archiveName, &downloadPath, &repoUrl
	if downloadGitArchive(md) == nil {
		checkMdFiles(md)
	}
	if md.MdFileList == nil {
		emptyState := "[INF] No markdown links were found."
		md.State = &emptyState
	}
	ch <- *md
	//generateMdReport(*md, os.Stdout)
}

func main() {
	execPath = "/tmp/github"
	githubAccount = "groovy-sky"
	var repos []*Repository
	// Query Github API for a public repository list
	resp, err := http.Get("https://api.github.com/users/" + githubAccount + "/repos?type=owner&per_page=100&type=public")
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	// Parse JSON to repository list
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		log.Fatalln(err)
	}

	path, err := os.Getwd()
	if err != nil {
		path = execPath
	}

	f, err := os.Create(filepath.Join(path, reportFileName))
	defer f.Close()
	reports := make(chan MdReport, len(repos))
	//CheckGitMdLinks(repos[9])
	//generateMdReport(<-reports)
	// Store and parse public and active repositories
	for i := range repos {
		if !*repos[i].Fork && !*repos[i].Disabled && !*repos[i].Archived {
			go CheckGitMdLinks(repos[i], reports)
			//CheckGitMdLinks(repos[i])
		}
	}
	for {
		generateMdReport(<-reports, f)
	}
}
