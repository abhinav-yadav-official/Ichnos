package crawler

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseHTMLExtractsTitleBodyAndLinks(t *testing.T) {
	parser := NewParser("https://example.com/articles/page")

	parsed, err := parser.Parse(strings.NewReader(`
		<html>
			<head><title>Example Article</title></head>
			<body>
				<article>
					<h1>Example Article</h1>
					<p>This article has enough readable text for extraction.</p>
					<p>It includes a second paragraph so readability has content.</p>
				</article>
				<a href="/about">About</a>
				<a href="https://other.example/path#section">Other</a>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.Title != "Example Article" {
		t.Fatalf("Title = %q", parsed.Title)
	}
	if !strings.Contains(parsed.Body, "second paragraph") {
		t.Fatalf("Body = %q", parsed.Body)
	}
	wantLinks := map[string]bool{
		"https://example.com/about":  false,
		"https://other.example/path": false,
	}
	for _, link := range parsed.Links {
		if _, ok := wantLinks[link]; ok {
			wantLinks[link] = true
		}
	}
	for link, found := range wantLinks {
		if !found {
			t.Fatalf("link %q not found in %#v", link, parsed.Links)
		}
	}
}

func TestLiveParserExtractsLinksFromHackerNews(t *testing.T) {
	if os.Getenv("ICHNOS_LIVE_TESTS") != "1" {
		t.Skip("set ICHNOS_LIVE_TESTS=1 to fetch live pages")
	}

	fetcher := NewFetcher("CrawlerBot/1.0", 10*time.Second)
	page, err := fetcher.Fetch(context.Background(), "https://news.ycombinator.com")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	parsed, err := NewParser(page.FinalURL).Parse(strings.NewReader(string(page.Body)))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(parsed.Links) == 0 {
		t.Fatal("expected at least one link")
	}
}

func TestLiveReadabilityReturnsBodyFromArticle(t *testing.T) {
	if os.Getenv("ICHNOS_LIVE_TESTS") != "1" {
		t.Skip("set ICHNOS_LIVE_TESTS=1 to fetch live pages")
	}

	fetcher := NewFetcher("CrawlerBot/1.0", 10*time.Second)
	page, err := fetcher.Fetch(context.Background(), "https://go.dev/blog/go1.22")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	parsed, err := NewParser(page.FinalURL).Parse(strings.NewReader(string(page.Body)))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.Body == "" {
		t.Fatal("readability body is empty")
	}
}
