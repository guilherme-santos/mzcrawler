package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/guilherme-santos/mzcrawler/http"
)

var defaultConcurrenctClients uint = 5

var (
	followSubDomains = flag.Bool("subdomains", false, "sets if should follow subdomains")
	verbose          = flag.Bool("v", false, "log the crawler progress")
	concurrent       = flag.Uint("n", defaultConcurrenctClients, "number of concurrent http calls")
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s <url>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	baseurl := flag.Arg(0)

	c, err := http.NewWebCrawler(baseurl, *concurrent)
	if err != nil {
		fmt.Printf("Unable to create a crawler to %s: %s\n", baseurl, err)
		os.Exit(1)
	}
	c.Verbose = *verbose
	c.FollowSubDomains = *followSubDomains

	sitemap, err := c.Crawl()
	if err != nil {
		fmt.Printf("Unable to crawl %s: %s\n", baseurl, err)
		os.Exit(1)
	}

	// Print sitemap returned.
	j, _ := json.MarshalIndent(sitemap, "", "   ")
	fmt.Println(string(j))
}
