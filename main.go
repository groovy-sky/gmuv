package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// https://github.com/google/go-github/blob/2d872b40760dcf7080786ece0a4735509ff071f4/github/repos.go#L28
type Repository struct {
	Name          *string `json:"name,omitempty"`
	URL           *string `json:"url,omitempty"`
	Fork          *bool   `json:"fork,omitempty"`
	Disabled      *bool   `json:"disabled,omitempty"`
	Archived      *bool   `json:"archived,omitempty"`
	CloneURL      *string `json:"clone_url,omitempty"`
	DefaultBranch *string `json:"default_branch,omitempty"`
}

func DownloadArchive(filepath, url string) (err error) {
	out, err := os.Create(filepath)

	if err != nil {
		return fmt.Errorf("[ERR] Couldn't initiate a download file/directory: %s", err)
	}

	defer out.Close()

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
	downloadLink := *r.URL + "/tarball/" + *r.DefaultBranch
	downloadPath := "/tmp/github/" + *r.Name + "tar.gz"
	fmt.Println(downloadLink)
	if err := DownloadArchive(downloadPath, downloadLink); err != nil {
		return err
	}
	return nil
}

func main() {
	var repos []*Repository
	resp, err := http.Get("https://api.github.com/users/groovy-sky/repos?type=owner&per_page=100&type=public")
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		log.Fatalln(err)
	}

	for i := range repos {
		if !*repos[i].Fork && !*repos[i].Disabled && !*repos[i].Archived {
			if err := CheckGitMdLinks(*&repos[i]); err != nil {
				fmt.Println(err)
			}
		}
	}

}
