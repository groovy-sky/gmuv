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

func getFileExtension(s string) string {
	ext := strings.Split(s, ".")
	return ext[len(ext)-1]
}

func checkMdLink(l string) error {
	// Cut text after (
	_, url, _ := strings.Cut(l, "(")
	// Delete last elemnt, which is )
	url = url[:len(url)-1]

	res, err := http.Get(url)
	fmt.Println(res.StatusCode)
	return err
}

func extractFiles(src string, f *zip.File) error {

	if !f.FileInfo().IsDir() {
		fileName := f.FileInfo().Name()
		relativePath, _, _ := strings.Cut(f.FileHeader.Name, fileName)
		relativePath = filepath.Clean(relativePath)
		_, relativePath, _ = strings.Cut(relativePath, "/")
		ext := getFileExtension(fileName)
		// Proceed if file is not a directory and has .md extension
		if strings.ToLower(ext) == "md" {
			relativeDestFile := filepath.Join(relativePath, fileName)
			fmt.Println(relativeDestFile)

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
				fmt.Printf("%s\n", v)
				fmt.Println(checkMdLink(string(v)))
			}

		}
	}
	return nil
}

func unzipArchive(src, zipName string) error {
	reader, err := zip.OpenReader(filepath.Join(src, zipName))
	if err != nil {
		return fmt.Errorf("[ERR] Couldn't open archive: %s", err)
	}

	defer reader.Close()
	for _, f := range reader.File {
		extractFiles(src, f)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("[ERR] Couldn't delete the folder: %s", err)
	}
	return nil
}

func ExctractMdFiles(filePath, zip string) (err error) {
	unzipArchive(filePath, zip)
	return nil
}

func DownloadGitArchive(downloadPath, fileName, url string) (err error) {
	fullpath := filepath.Join(downloadPath, fileName)

	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		return fmt.Errorf("[ERR] Couldn't create filepath: %s", err)
	}

	out, err := os.Create(fullpath)

	if err != nil {
		return fmt.Errorf("[ERR] Couldn't initiate a download file/directory: %s", err)
	}

	defer out.Close()
	fmt.Println("[INF] Downloading " + url)
	resp, err := http.Get(url)

	if err != nil {
		return fmt.Errorf("[ERR] Couldn't download a file: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[ERR] Couldn't download a file: %s", resp.Status)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("[ERR] Couldn't store a file: %s", err)
	}

	return nil
}

func CheckGitMdLinks(r *Repository) (err error) {
	downloadLink := *r.HTMLURL + "/archive/refs/heads/" + *r.DefaultBranch + ".zip"
	archiveName := *r.Name + ".zip"
	downloadPath := filepath.Join(defaultPath, *r.Name)
	if err := DownloadGitArchive(downloadPath, archiveName, downloadLink); err != nil {
		return err
	}
	ExctractMdFiles(downloadPath, archiveName)
	return nil
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

	if err := CheckGitMdLinks(*&repos[0]); err != nil {
		fmt.Println(err)
	}
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
