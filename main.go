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

// Part of Github API response strutures
// https://github.com/google/go-github/blob/2d872b40760dcf7080786ece0a4735509ff071f4/github/repos.go#L28
type Repository struct {
	Name          *string `json:"name,omitempty"`
	URL           *string `json:"url,omitempty"`
	Fork          *bool   `json:"fork,omitempty"`
	Disabled      *bool   `json:"disabled,omitempty"`
	Archived      *bool   `json:"archived,omitempty"`
	CloneURL      *string `json:"clone_url,omitempty"`
	HTMLURL       *string `json:"html_url,omitempty"`
	DefaultBranch *string `json:"default_branch,omitempty"`
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

func checkMdLink(l string) {
	// Delete last elemnt, which is )
	l = l[:len(l)-1]
	// Search for URL using regexp
	re := regexp.MustCompile(`(https?:\/\/)?([\da-z\.-]+)\.([a-z\.]{2,6})([\/\w\.-]*)*\/?$`)
	url := re.FindString(l)
	if (l[0]) == '/' {
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

func extractAndCheckMdFiles(src string, f *zip.File) error {

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
			for _, v := range matches {
				checkMdLink(string(v))
			}

		}
	}
	return nil
}

func ExctractMdFiles(src, zipName string, m *MdReport) error {
	reader, err := zip.OpenReader(filepath.Join(src, zipName))
	if err != nil {
		return fmt.Errorf("[ERR] Couldn't open archive: %s", err)
	}

	defer reader.Close()
	for _, f := range reader.File {
		extractAndCheckMdFiles(src, f)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("[ERR] Couldn't delete the folder: %s", err)
	}
	return nil
}

func DownloadGitArchive(md *MdReport) {
	fullpath := filepath.Join(*md.ZipPath, *md.ZipName)

	if err := os.MkdirAll(*md.ZipPath, 0755); err != nil {
		*md.State = ("[ERR] Couldn't create " + *md.ZipPath + " path.\n\t" + err.Error())
		return
	}

	out, err := os.Create(fullpath)

	if err != nil {
		*md.State = ("[ERR] Couldn't create " + fullpath + " file.\n\t" + err.Error())
		return
	}

	defer out.Close()
	resp, err := http.Get(*md.ZipUrl)

	if err != nil {
		*md.State = ("[ERR] Couldn't download " + *md.ZipUrl + " file.\n\t" + err.Error())
		return
	}

	defer resp.Body.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		*md.State = ("[ERR] Couldn't store downloaded file.\n\t" + err.Error())
		return
	}
}

func CheckGitMdLinks(r *Repository) {
	//repoUrl := (*r.HTMLURL + "/blob/" + *r.DefaultBranch)
	var md MdReport
	md.Repository = r
	downloadLink := *r.HTMLURL + "/archive/refs/heads/" + *r.DefaultBranch + ".zip"
	archiveName := *r.Name + ".zip"
	downloadPath := filepath.Join(defaultPath, *r.Name)
	md.ZipUrl, md.ZipName, md.ZipPath = &downloadLink, &archiveName, &downloadPath
	DownloadGitArchive(&md)
	ExctractMdFiles(downloadPath, archiveName, &md)
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
