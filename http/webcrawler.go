package http

import (
	"bytes"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/guilherme-santos/mzcrawler"
	"golang.org/x/net/html"
)

// ClientTimeout defines the timeout when do http calls.
var ClientTimeout = 5 * time.Second

// WebCrawler is an http implementation of mzcrawler.WebCrawler.
type WebCrawler struct {
	urlstr           string
	url              *url.URL
	domain           string
	sitemap          mzcrawler.Sitemap
	sitemapMu        sync.Mutex
	Logger           *log.Logger
	HTTPClient       *http.Client
	Verbose          bool
	FollowSubDomains bool
}

// NewWebCrawler creates new instance of http.WebCrawler.
func NewWebCrawler(baseurl string) (*WebCrawler, error) {
	u, err := url.Parse(baseurl)
	if err != nil {
		return nil, err
	}

	return &WebCrawler{
		urlstr:  baseurl,
		url:     u,
		domain:  domain(u),
		sitemap: make(mzcrawler.Sitemap),
		Logger:  log.New(os.Stdout, "http.webcrawler: ", 0),
		HTTPClient: &http.Client{
			Timeout: ClientTimeout,
		},
		FollowSubDomains: true,
	}, nil
}

type logRecord map[string]interface{}

func (c *WebCrawler) log(msg string, data logRecord) {
	if c.Verbose {
		var buf bytes.Buffer
		buf.WriteString("timestamp=")
		buf.WriteString(time.Now().UTC().String())
		buf.WriteString(" msg=")
		buf.WriteString(msg)

		for k, v := range data {
			buf.WriteString(" ")
			buf.WriteString(k)
			buf.WriteString("=")
			switch v.(type) {
			case string:
				buf.WriteString(v.(string))
			}
		}

		c.Logger.Println(buf.String())
	}
}

func (c *WebCrawler) Crawl() (mzcrawler.Sitemap, error) {
	urlCh, err := c.crawlURL(c.url)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	c.worker(&wg, c.urlstr, urlCh)

	// Wait all gourotines workers spawn finish.
	wg.Wait()
	return c.sitemap, nil
}

// newURLFound checks if URL was visited already, case not it'll
// add it in the list of visited URLs.
func (c *WebCrawler) newURLFound(urlstr string) bool {
	c.sitemapMu.Lock()
	defer c.sitemapMu.Unlock()

	if _, ok := c.sitemap[urlstr]; ok {
		return false
	}

	c.sitemap[urlstr] = make([]string, 0)
	return true
}

func (c *WebCrawler) worker(wg *sync.WaitGroup, urlstr string, urlCh chan string) {
	if !c.newURLFound(urlstr) {
		return
	}

	urls := make(map[string]struct{})

	for u := range urlCh {
		u := c.normalizeURL(u)

		if _, ok := urls[u]; !ok {
			urls[u] = struct{}{}
		}

		if !c.shouldFollow(u) {
			continue
		}

		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			urlCh, err := c.crawlURL(c.url)
			if err == nil {
				c.worker(wg, u, urlCh)
			}
		}(u)
	}

	// create sitemap to urlstr and save it.
	sitemap := func() (res []string) {
		for u := range urls {
			res = append(res, u)
		}
		return
	}()

	c.sitemapMu.Lock()
	c.sitemap[urlstr] = sitemap
	c.sitemapMu.Unlock()

	return nil
}

// crawlURL calls baseurl and return an channel that will be send all
// urls founds in the baseurl.
func (c *WebCrawler) crawlURL(baseurl string) (chan string, error) {
	c.log("crawling...", logRecord{"url": baseurl})

	urlCh := make(chan string)

	resp, err := c.HTTPClient.Get(baseurl)
	if err != nil {
		close(urlCh)
		return nil, err
	}

	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		close(urlCh)
		return nil, err
	}

	go func() {
		// goroutine to handle the html and extract new urls.
		var fn func(*html.Node)
		fn = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						val := strings.TrimSpace(a.Val)
						if val == "" || strings.HasPrefix(val, "#") {
							continue
						}

						urlCh <- val
						break
					}
				}
			}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				fn(c)
			}
		}
		fn(doc)
		// url fully parsed, we can close the channel.
		close(urlCh)
	}()

	return urlCh, nil
}

func (c *WebCrawler) shouldFollow(baseurl string) bool {
	u, err := url.Parse(baseurl)
	if err != nil {
		return false
	}

	// TODO it'll be nice check robots.txt to avoid crawl URL that shouldn't.

	if !c.FollowSubDomains {
		return strings.EqualFold(u.Host, c.domain)
	}

	return strings.HasSuffix(u.Host, c.domain)
}

func (c *WebCrawler) normalizeURL(u string) string {
	u = strings.TrimSuffix(u, "/")

	if u == "" {
		newurl := new(url.URL)
		*newurl = *c.url
		newurl.Path = ""
		newurl.RawQuery = ""
		newurl.Fragment = ""
		return newurl.String()
	}

	if strings.HasPrefix(u, "//") {
		return c.url.Scheme + ":" + u
	}

	if isReletivePath(u) {
		newurl := new(url.URL)
		*newurl = *c.url
		newurl.Path = u
		newurl.RawQuery = ""
		newurl.Fragment = ""
		return newurl.String()
	}

	return u
}

func isReletivePath(path string) bool {
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "..")
}

func domain(baseurl *url.URL) string {
	hostparts := strings.SplitAfter(baseurl.Host, ".")
	if len(hostparts) <= 2 {
		return baseurl.Host

	}
	return strings.Join(hostparts[len(hostparts)-2:], "")
}
