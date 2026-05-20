package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFetchHTMLReturnsBodyAndFinalURL(t *testing.T) {
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.UserAgent()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Hello</title></head><body>World</body></html>"))
	}))
	t.Cleanup(server.Close)

	fetcher := NewFetcher("CrawlerBot/1.0", time.Second)
	page, err := fetcher.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if page.URL != server.URL {
		t.Fatalf("URL = %q, want %q", page.URL, server.URL)
	}
	if page.FinalURL != server.URL {
		t.Fatalf("FinalURL = %q, want %q", page.FinalURL, server.URL)
	}
	if page.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", page.StatusCode, http.StatusOK)
	}
	if string(page.Body) == "" {
		t.Fatal("Body is empty")
	}
	if userAgent != "CrawlerBot/1.0" {
		t.Fatalf("User-Agent = %q", userAgent)
	}
}

func TestFetchSkipsNonHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	fetcher := NewFetcher("CrawlerBot/1.0", time.Second)
	_, err := fetcher.Fetch(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if err != ErrNonHTML {
		t.Fatalf("error = %v, want ErrNonHTML", err)
	}
}

func TestFetchRecordsFinalURLAfterRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>final</body></html>"))
	}))
	t.Cleanup(final.Close)

	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	t.Cleanup(start.Close)

	fetcher := NewFetcher("CrawlerBot/1.0", time.Second)
	page, err := fetcher.Fetch(context.Background(), start.URL)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if page.URL != start.URL {
		t.Fatalf("URL = %q, want %q", page.URL, start.URL)
	}
	if page.FinalURL != final.URL {
		t.Fatalf("FinalURL = %q, want %q", page.FinalURL, final.URL)
	}
}

func TestLiveFetchExampleDotCom(t *testing.T) {
	if os.Getenv("ICHNOS_LIVE_TESTS") != "1" {
		t.Skip("set ICHNOS_LIVE_TESTS=1 to fetch live pages")
	}

	fetcher := NewFetcher("CrawlerBot/1.0", 10*time.Second)
	page, err := fetcher.Fetch(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if page.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", page.StatusCode, http.StatusOK)
	}
	parsed, err := NewParser(page.FinalURL).Parse(strings.NewReader(string(page.Body)))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.Title == "" {
		t.Fatal("title is empty")
	}
}
