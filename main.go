package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// https://github.com/google/go-github/blob/2d872b40760dcf7080786ece0a4735509ff071f4/github/repos.go#L28
type Repository struct {
	Name     *string `json:"name,omitempty"`
	URL      *string `json:"url,omitempty"`
	Fork     *bool   `json:"fork,omitempty"`
	Disabled *bool   `json:"disabled,omitempty"`
	Archived *bool   `json:"archived,omitempty"`
}

func CheckGitMdLinks(url, name string) {
	fmt.Println(url, name)
}

func main() {
	var repos []*Repository
	resp, err := http.Get("https://api.github.com/users/groovy-sky/repos?type=owner&per_page=100")
	if err != nil {
		log.Fatalln(err)
	}

	resp.Header.Set("Accept", "application/json")
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		log.Fatalln(err)
	}

	for i := range repos {
		if !*repos[i].Fork && !*repos[i].Disabled && !*repos[i].Archived {
			fmt.Printf("%s\n", *repos[i].URL)
		}
	}

}
