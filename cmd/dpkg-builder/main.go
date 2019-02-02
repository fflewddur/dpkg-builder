package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli"
	"golang.org/x/net/html"
)

// dpkg-builder takes a package name and attempts to automatically build a
// .deb using the source files from debian-testing.

const debianTestingBaseURL string = "https://packages.debian.org/buster/"

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
		download(linkURL, pkgName)
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
		case strings.HasSuffix(l, ".orig.tar.xz") || strings.HasSuffix(l, ".orig.tar.gz"):
			filteredLinks = append(filteredLinks, l)
		case strings.HasSuffix(l, ".debian.tar.xz"):
			filteredLinks = append(filteredLinks, l)
		}
	}
	return
}

func download(u *url.URL, pkgName string) {
	path, dir := buildPath(u, pkgName)
	log.Printf("Downloading %s to %s...", u, path)
	ensureDirExists(dir)
	if fileExists(path) {
		log.Printf("File already exists, skipping.")
		return
	}

	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Error creating file %s: %s", path, err)
	}
	defer file.Close()

	resp, err := http.Get(u.String())
	if err != nil {
		log.Fatalf("Error downloading file %s: %s", u, err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Fatalf("Error downloading file %s to %s: %s", u, path, err)
	}
}

func buildPath(u *url.URL, pkg string) (path string, parentDir string) {
	i := strings.LastIndex(u.Path, "/")
	parentDir = filepath.FromSlash(pkg)
	path = filepath.Join(parentDir, filepath.FromSlash(u.Path[i+1:]))
	return
}

func ensureDirExists(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.Mkdir(dir, 0755)
		if err != nil {
			log.Fatalf("Error creating directory %s: %s", dir, err)
		}
	}
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}
