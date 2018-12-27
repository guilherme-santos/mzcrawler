package mzcrawler

// WebCrawler defines an interface to crawl an website.
type WebCrawler interface {
	// Crawl returns a map with all URLs visited and the list of
	// urls found in each one.
	Crawl() (Sitemap, error)
}

type Sitemap map[string][]string
