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
	semaphore        chan struct{}
	Logger           *log.Logger
	HTTPClient       *http.Client
	Verbose          bool
	FollowSubDomains bool
}

// NewWebCrawler creates new instance of http.WebCrawler.
func NewWebCrawler(baseurl string, concurrent uint) (*WebCrawler, error) {
	u, err := url.Parse(baseurl)
	if err != nil {
		return nil, err
	}

	return &WebCrawler{
		urlstr:    baseurl,
		url:       u,
		domain:    domain(u),
		sitemap:   make(mzcrawler.Sitemap),
		semaphore: make(chan struct{}, concurrent),
		Logger:    log.New(os.Stdout, "http.webcrawler: ", 0),
		HTTPClient: &http.Client{
			Timeout: ClientTimeout,
		},
		Verbose:          false,
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
	var wg sync.WaitGroup

	err := c.worker(&wg, c.urlstr)
	if err != nil {
		return nil, err
	}

	// Wait read from all crawled URL to return.
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

func (c *WebCrawler) worker(wg *sync.WaitGroup, urlstr string) error {
	if !c.newURLFound(urlstr) {
		c.log("url visited already, ignoring...", logRecord{"url": urlstr})
		return nil
	}

	// crawl urlstr
	urlCh, err := c.crawlURL(urlstr)
	if err != nil {
		return err
	}

	// urls it's a set of URL (avoiding duplicated).
	urls := make(map[string]struct{})

	for u := range urlCh {
		urlstr := c.normalizeURL(u)

		if _, ok := urls[urlstr]; !ok {
			urls[urlstr] = struct{}{}
		}

		if !c.shouldFollow(urlstr) {
			c.log("url shouldn't be followed", logRecord{"url": urlstr})
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(wg, urlstr)
		}()
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
	// try to acquire one spot. it'll block until
	// at least one spot is available.
	c.semaphore <- struct{}{}
	defer func() {
		// When finish the http call release the stop occupied.
		<-c.semaphore
	}()

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

func (c *WebCrawler) normalizeURL(urlstr string) string {
	urlstr = strings.TrimSuffix(urlstr, "/")
	if urlstr == "" {
		return c.urlstr
	}

	if strings.HasPrefix(urlstr, "//") {
		return c.url.Scheme + ":" + urlstr
	}

	if isReletivePath(urlstr) {
		newurl := new(url.URL)
		*newurl = *c.url
		if !strings.HasPrefix(urlstr, "/") {
			newurl.Path += "/"
		}
		newurl.Path += urlstr
		return newurl.String()
	}

	return urlstr
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
