package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// https://github.com/google/go-github/blob/2d872b40760dcf7080786ece0a4735509ff071f4/github/repos.go#L28
type Repository struct {
	ID   *int64  `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
	URL  *string `json:"url,omitempty"`
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
		fmt.Printf("%s\n", *repos[i].URL)
	}

}
