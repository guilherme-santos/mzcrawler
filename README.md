# MZCrawler

## Building

To build the webcrawler you need to have [Go](https://golang.org/doc/install) instaled in your machine, you can check it typing `go version`.

Once you have Go installed and working you can type the following command to build the app.
```
go build -o mzcrawler cmd/mzcrawler/main.go
```

## Running

If you already have the binaries in your machine you can execute it passing the URL to be crawled, like:
```
./mzcrawler https://monzo.com
```

* By default subdomains (e.g. https://web.monzo.com) are not followed, you need to explicit pass `-subdomains` flag to be able to follow them.
* By default this crawler does 5 concurrent requests, to change this number you can pass `-n` and the number of concurrent requests.

You also can check the crawling progress passing `-v` flag.

A full example would be:
```
./mzcrawler -v -n 10 -subdomains https://monzo.com
```

## Testing

You can run the unit tests using the following command.
```
go test -v ./...
```
