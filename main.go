package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var defaultPath, githubAccount string


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
	WebUrl			*string // for relative paths check
}

// Checked URL structure
type MdLink struct {
	Path  *string
	Link  *string
	State *string
}

// Generated reports structure
type MdReport struct {
	Repository *Repository
	MdFile     *map[string]*map[MdLink]*string
	ZipUrl     *string
	ZipName    *string
	ZipPath    *string
	State      *string
}

func getFileExtension(s string) string {
	ext := strings.Split(s, ".")
	return ext[len(ext)-1]
}

func checkMdLink(md *MdReport, l string) {
	// Delete last elemnt, which is a brace
	l = l[:len(l)-1]
	// Delete part containing square brackets and brace, which comes before a link
	l = l[len(regexp.MustCompile(`(^\[(.*?)]\()`).FindString(l)):]
	// Check if link starts with http/https
	url := regexp.MustCompile(`(^https?:\/\/)?([\da-z\.-]+)\.([a-z\.]{2,6})([\/\w\.-]*)*\/?$`).FindString(l)
	fmt.Println(l + " | " + url)
	// If 
	if url != "" {
		//url = *md.Repository.WebUrl + url
		return
	}
	res, err := http.Get(url)
	if err == nil {
		defer res.Body.Close()
		if res.StatusCode > 299 {
			fmt.Printf("[ERR] Response from %s: %d\n", url, res.StatusCode)
		} else {
			fmt.Printf("[INF] Response from %s: %d\n", url, res.StatusCode)
		}
	} else if strings.Contains(err.Error(), "unsupported protocol scheme") {
		fmt.Printf("[ERR] Missing protocol (http/https) for %s\n", url)
	} else if strings.Contains(err.Error(), "dial tcp: lookup") {
		fmt.Printf("[ERR] Couldn't resolve %s\n", url)
	} else {
		fmt.Printf("[ERR] %s", err)
	}

}

func findAndCheckMdFile(md *MdReport, f *zip.File) error {

	if !f.FileInfo().IsDir() {
		fileName := f.FileInfo().Name()
		ext := getFileExtension(fileName)
		// Proceed if file is not a directory and has .md extension
		if strings.ToLower(ext) == "md" {
			//relativeDestFile := filepath.Join(relativePath, fileName)

			zipContent, err := f.Open()
			if err != nil {
				return fmt.Errorf("[ERR] Couldn't read archive's content: %s", err)
			}
			defer zipContent.Close()

			content, err := ioutil.ReadAll(zipContent)
			if err != nil {
				return fmt.Errorf("[ERR] Couldn't load archive's content: %s", err)
			}

			// Use regexp for matching Markdown URL
			re := regexp.MustCompile(`\[[^\[\]]*?\]\(.*?\)|^\[*?\]\(.*?\)`)
			matches := re.FindAll(content, -1)
			for _, val := range matches {
				checkMdLink(md, string(val))
			}

		}
	}
	return nil
}

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

func CheckGitMdLinks(r *Repository) {
	var md MdReport
	md.Repository = r
	downloadLink := *r.HTMLURL + "/archive/refs/heads/" + *r.DefaultBranch + ".zip"
	archiveName := *r.Name + ".zip"
	downloadPath := filepath.Join(defaultPath, *r.Name)
	repoUrl := (*r.HTMLURL + "/blob/" + *r.DefaultBranch)
	md.ZipUrl, md.ZipName, md.ZipPath , md.Repository.WebUrl = &downloadLink, &archiveName, &downloadPath, &repoUrl
	if downloadGitArchive(&md) == nil {
		checkMdFiles(&md)
	}
}

func main() {
	defaultPath = "/tmp/github"
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

	CheckGitMdLinks(*&repos[0])
	// Store and parse public and active repositories
	/*
		for i := range repos {
			if !*repos[i].Fork && !*repos[i].Disabled && !*repos[i].Archived {
				if err := CheckGitMdLinks(*&repos[i]); err != nil {
					fmt.Println(err)
				}
			}
		}
	*/
}
