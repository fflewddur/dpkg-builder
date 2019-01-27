package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/urfave/cli"
	"golang.org/x/net/html"
)

// dpkg-builder takes a package name and attempts to automatically build a
// .deb using the source files from debian-testing.

const debianTestingBaseURL string = "https://packages.debian.org/buster/"

var pkgName string

func main() {
	app := cli.NewApp()
	app.Name = "dpkg-builder"
	app.Usage = "Build a package from debian-testing"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name:   "build",
			Usage:  "Download and build the package",
			Action: commandBuild,
		},
		{
			Name:   "fetch",
			Usage:  "Only download the package files",
			Action: commandFetch,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func commandBuild(c *cli.Context) error {
	log.Println("build")
	pkgName, ok := validateArgs(c)
	if !ok {
		return errors.New("fetch: no package name provided")
	}
	log.Print("Building " + pkgName)

	return nil
}

func commandFetch(c *cli.Context) error {
	pkgName, ok := validateArgs(c)
	if !ok {
		return errors.New("fetch: no package name provided")
	}
	log.Print("Fetching " + pkgName)

	baseURL, err := url.Parse(debianTestingBaseURL)
	if err != nil {
		log.Fatalf("Error parsing base URL: %s", err)
	}

	fetchURL := fmt.Sprintf("%s%s", baseURL, url.PathEscape(pkgName))
	log.Printf("Downloading %s...", fetchURL)
	resp, err := http.Get(fetchURL)
	if err != nil {
		log.Fatalf("Error getting %s: %s", fetchURL, err)
	}
	defer resp.Body.Close()

	links := getLinks(resp.Body)
	links = filterLinks(links)
	for _, l := range links {
		linkURL, err := url.Parse(l)
		if err != nil {
			log.Fatalf("Error parsing URL %s: %s", l, err)
		}
		if !linkURL.IsAbs() {
			linkURL = baseURL.ResolveReference(linkURL)
		}
		download(linkURL)
	}
	return nil
}

func validateArgs(c *cli.Context) (name string, ok bool) {
	if c.NArg() > 0 {
		name = c.Args()[0]
		ok = true
	}

	return
}

func getLinks(body io.ReadCloser) (links []string) {
	tokenizer := html.NewTokenizer(body)
	for {
		tagToken := tokenizer.Next()
		if tagToken == html.ErrorToken {
			break
		} else if tagToken == html.StartTagToken {
			token := tokenizer.Token()
			isAnchor := token.Data == "a"
			if isAnchor {
				if href, ok := getHref(token); ok {
					links = append(links, href)
				}
			}
		}
	}
	return
}

func getHref(t html.Token) (href string, ok bool) {
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
			break
		}
	}
	return
}

func filterLinks(links []string) (filteredLinks []string) {
	for _, l := range links {
		switch {
		case strings.HasSuffix(l, ".dsc"):
			filteredLinks = append(filteredLinks, l)
		case strings.HasSuffix(l, ".orig.tar.xz"):
			filteredLinks = append(filteredLinks, l)
		case strings.HasSuffix(l, ".debian.tar.xz"):
			filteredLinks = append(filteredLinks, l)
		}
	}
	return
}

func download(u *url.URL) {
	log.Printf("Downloading %s...", u)
	return
}
