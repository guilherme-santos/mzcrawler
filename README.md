# MZCrawler

## Building

To build the webcrawler you need to have [Go](https://golang.org/doc/install) instaled in your machine, you can check it typing `go version`.

Once you have Go installed in working you can type following command to build the app.
```
go build -o mzcrawler cmd/mzcrawler/main.go
```

## Running

If you already have the binaries in your machine you can execute it passing the URL to be crawled, like:
```
./mzcrawler https://monzo.com
```

* By default subdomains (e.g. https://web.monzo.com) are not followed, you need to explicit pass `-subdomains` flag to be able to follow them.

You also can pass `-v` flag to show all URLs visited and also the ones ignored.

## Testing

You can run unit tests using the following command.
```
go test -v ./...
```
