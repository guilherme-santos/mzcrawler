package http

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/guilherme-santos/mzcrawler"
)

func noError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("No error expected but got: %s", err)
	}
}

func assert(t *testing.T, expected, received interface{}) bool {
	if !reflect.DeepEqual(expected, received) {
		t.Errorf("Expected %v but got %v", expected, received)
		return false
	}
	return true
}

func TestDomain(t *testing.T) {
	testcases := []struct {
		URL        string
		MainDomain string
	}{
		{URL: "http://localhost", MainDomain: "localhost"},
		{URL: "http://monzo.com", MainDomain: "monzo.com"},
		{URL: "https://blog.monzo.com", MainDomain: "monzo.com"},
		{URL: "https://secure.blog.monzo.com", MainDomain: "monzo.com"},
	}

	for _, tc := range testcases {
		u, _ := url.Parse(tc.URL)
		assert(t, tc.MainDomain, domain(u))
	}
}

func TestNormalizeURL(t *testing.T) {
	testcases := []struct {
		URL        string
		Normalized string
	}{
		{URL: "/", Normalized: "https://monzo.com"},
		{URL: "/blog", Normalized: "https://monzo.com/blog"},
		{URL: "../blog", Normalized: "https://monzo.com/../blog"},
		{URL: "//secure.monzo.com/blog", Normalized: "https://secure.monzo.com/blog"},
		{URL: "http://monzo.com/blog", Normalized: "http://monzo.com/blog"},
	}

	c, err := NewWebCrawler("https://monzo.com/path?query=param#fragment")
	noError(t, err)

	for _, tc := range testcases {
		if !assert(t, tc.Normalized, c.normalizeURL(tc.URL)) {
			t.Logf("URL %s", tc.URL)
		}
	}
}

func TestShouldFollow(t *testing.T) {
	testcases := []struct {
		URL              string
		FollowSubDomains bool
		ShouldFollow     bool
	}{
		{URL: "https://localhost", FollowSubDomains: false, ShouldFollow: false},
		{URL: "https://localhost", FollowSubDomains: true, ShouldFollow: false},
		{URL: "https://monzo.com/blog", FollowSubDomains: false, ShouldFollow: true},
		{URL: "https://monzo.com/blog", FollowSubDomains: true, ShouldFollow: true},
		{URL: "https://blog.monzo.com/article", FollowSubDomains: false, ShouldFollow: false},
		{URL: "https://blog.monzo.com/article", FollowSubDomains: true, ShouldFollow: true},
	}

	c, err := NewWebCrawler("https://monzo.com")
	noError(t, err)

	for _, tc := range testcases {
		c.FollowSubDomains = tc.FollowSubDomains

		if !assert(t, tc.ShouldFollow, c.shouldFollow(tc.URL)) {
			t.Logf("URL %s with FollowSubDomains %v", tc.URL, tc.FollowSubDomains)
		}
	}
}

func TestCrawlURL_ExtractAllHref(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		f, err := os.Open(path.Join("testdata", "github.html"))
		noError(t, err)
		io.Copy(w, f)
	}))
	defer ts.Close()

	c, err := NewWebCrawler(ts.URL)
	noError(t, err)

	u, _ := url.Parse(ts.URL)
	urlCh, err := c.crawlURL(u)
	noError(t, err)

	var total int
	for range urlCh {
		total++
	}
	assert(t, 88, total)
}

func TestCrawler(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.RequestURI {
		case "/", "/about", "contact":
			fmt.Fprintf(w, `<a href="/"><a href="/about"><a href="%s/contact"><a href="https://fb.com/company">`, ts.URL)
		default:
			t.Errorf("%s is not an expected", req.RequestURI)
		}
	}))
	defer ts.Close()

	c, err := NewWebCrawler(ts.URL)
	noError(t, err)

	sitemap, err := c.Crawl()
	noError(t, err)

	testCrawlerSitemap(t, sitemap, ts.URL, "")
	testCrawlerSitemap(t, sitemap, ts.URL, "/about")
	testCrawlerSitemap(t, sitemap, ts.URL, "/contact")
}

func testCrawlerSitemap(t *testing.T, sitemap mzcrawler.Sitemap, baseurl, path string) {
	urls, ok := sitemap[baseurl+path]
	assert(t, true, ok)
	assert(t, 4, len(urls))

	expectedUrls := map[string]struct{}{
		baseurl:                  struct{}{},
		baseurl + "/about":       struct{}{},
		baseurl + "/contact":     struct{}{},
		"https://fb.com/company": struct{}{},
	}
	for _, v := range urls {
		if _, ok := expectedUrls[v]; !ok {
			t.Errorf("%s is not expected to be found", v)
		}
		delete(expectedUrls, v)
	}
	assert(t, 0, len(expectedUrls))
}
