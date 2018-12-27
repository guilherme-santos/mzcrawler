package http

import (
	"net/http"
	"net/url"
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
	HTTPClient       *http.Client
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
		HTTPClient: &http.Client{
			Timeout: ClientTimeout,
		},
		FollowSubDomains: true,
	}, nil
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
}

// crawlURL calls baseurl and return an channel that will be send all
// urls founds in the baseurl.
func (c *WebCrawler) crawlURL(baseurl *url.URL) (chan string, error) {
	urlCh := make(chan string)

	resp, err := c.HTTPClient.Get(baseurl.String())
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
