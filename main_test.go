package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestMarkdownLinksParsesMarkdownNodes(t *testing.T) {
	content := []byte(`
[inline](https://example.com/inline "title")
[reference][ref]
[ref]: https://example.com/reference
<https://example.com/autolink>
![image](https://example.com/image.png)
` + "`[not a link](https://example.com/code)`")

	want := []string{
		"https://example.com/inline",
		"https://example.com/reference",
		"https://example.com/autolink",
		"https://example.com/image.png",
	}
	if got := markdownLinks(content); !reflect.DeepEqual(got, want) {
		t.Fatalf("markdownLinks() = %v, want %v", got, want)
	}
}

func TestCheckURLAcceptsSuccessfulAndRedirectResponses(t *testing.T) {
	for _, status := range []int{
		http.StatusOK,
		http.StatusNoContent,
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusNotFound,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()

			_, got := checkURL(server.URL, linkHTTPClient)
			want := status >= http.StatusOK && status < http.StatusMultipleChoices
			if got != want {
				t.Errorf("checkURL() = %t, want %t for %d", got, want, status)
			}
		})
	}
}

func TestDownloadGitArchiveRecordsErrorWithoutPanic(t *testing.T) {
	md := &MdReport{
		ZipPath: "/dev/null/gmuv",
		ZipName: "archive.zip",
		ZipURL:  "https://example.com/archive.zip",
	}

	if err := downloadGitArchive(md); err == nil {
		t.Fatal("downloadGitArchive() error = nil, want an error")
	}
	if md.State == "" {
		t.Fatal("downloadGitArchive() did not record an error state")
	}
}
