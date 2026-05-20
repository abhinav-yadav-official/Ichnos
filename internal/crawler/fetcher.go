package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNonHTML = errors.New("response is not HTML")

type Fetcher struct {
	client    *http.Client
	userAgent string
}

type FetchedPage struct {
	URL        string
	StatusCode int
	Body       []byte
	FinalURL   string
}

func NewFetcher(userAgent string, timeout time.Duration) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: timeout,
		},
		userAgent: userAgent,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (FetchedPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return FetchedPage{}, err
	}
	req.Header.Set("User-Agent", f.userAgent)

	res, err := f.client.Do(req)
	if err != nil {
		return FetchedPage{}, err
	}
	defer res.Body.Close()

	contentType := res.Header.Get("Content-Type")
	if contentType != "" && !isHTMLContentType(contentType) {
		return FetchedPage{}, ErrNonHTML
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return FetchedPage{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return FetchedPage{}, fmt.Errorf("fetch %s: status %d", rawURL, res.StatusCode)
	}

	return FetchedPage{
		URL:        rawURL,
		StatusCode: res.StatusCode,
		Body:       body,
		FinalURL:   res.Request.URL.String(),
	}, nil
}

func isHTMLContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "text/html" || mediaType == "application/xhtml+xml"
}
