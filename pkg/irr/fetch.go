package irr

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jlaffaye/ftp"
)

func FetchFile(ctx context.Context, src string) ([]Route, error) {
	file, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("os.Open(): %s: %w", src, err)
	}
	defer file.Close()
	routes, err := decompress(file, src)
	if err != nil {
		return nil, fmt.Errorf("parse(): %w", err)
	}
	return routes, nil
}

func FetchURL(ctx context.Context, src string) ([]Route, error) {
	urlParsed, err := url.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("url.Parse(): %s: %w", src, err)
	}

	var data io.ReadCloser
	switch strings.ToLower(urlParsed.Scheme) {
	case "http", "https":
		req, err := http.NewRequestWithContext(ctx, "GET", src, nil)
		if err != nil {
			return nil, fmt.Errorf("http.NewRequest(): %s: %w", src, err)
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http.Get(): %s: %w", src, err)
		}
		defer resp.Body.Close()
		data = resp.Body
	case "ftp":
		conn, err := ftp.Dial(net.JoinHostPort(urlParsed.Host, "21"), ftp.DialWithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("ftp.Dial(): %s: %w", src, err)
		}
		if err := conn.Login("anonymous", "anonymous"); err != nil {
			return nil, fmt.Errorf("ftp.Login(): %s: %w", src, err)
		}
		r, err := conn.Retr(urlParsed.Path)
		if err != nil {
			return nil, fmt.Errorf("ftp.Retr(): %s: %w", src, err)
		}
		data = r
		defer conn.Quit()
	default:
		return nil, fmt.Errorf("Unknown scheme: %s", src)
	}

	routes, err := decompress(data, src)
	if err != nil {
		return nil, fmt.Errorf("parse(): %w", err)
	}
	return routes, nil
}

func decompress(reader io.ReadCloser, src string) ([]Route, error) {
	// decompress if needed
	data := reader
	if strings.HasSuffix(src, ".gz") {
		ungzData, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("gzip.NewReader(): %s: %w", src, err)
		}
		defer ungzData.Close()
		data = ungzData
	}

	// unmarshal the data into the format we want
	routes, err := ParseRoutes(data)
	if err != nil {
		return nil, fmt.Errorf("irr.ParseRoutes(): %s: %w", src, err)
	}

	return routes, nil
}
