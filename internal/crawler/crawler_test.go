package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestCrawlerProcessesSeedPublishesPageAndQueuesLinks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`
				<html>
					<head><title>Seed Page</title></head>
					<body>
						<article>
							<h1>Seed Page</h1>
							<p>This page has enough readable text for the crawler test.</p>
						</article>
						<a href="/next">Next</a>
					</body>
				</html>
			`))
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = server.URL
	t.Cleanup(server.Close)

	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	crawler := NewCrawler(CrawlerOptions{
		Client:      client,
		Frontier:    NewFrontier(client, "crawler:frontier"),
		Seen:        NewSeenSet(1000, 0.0001),
		Politeness:  NewPoliteness(time.Millisecond, "CrawlerBot/1.0", nil),
		Fetcher:     NewFetcher("CrawlerBot/1.0", time.Second),
		SeedURLs:    []string{serverURL},
		MaxDepth:    1,
		WorkerCount: 1,
		StreamName:  "pages",
	})

	if err := crawler.Seed(ctx); err != nil {
		t.Fatalf("Seed returned error: %v", err)
	}
	if err := crawler.CrawlOne(ctx); err != nil {
		t.Fatalf("CrawlOne returned error: %v", err)
	}

	streamLen, err := client.XLen(ctx, "pages").Result()
	if err != nil {
		t.Fatalf("XLen returned error: %v", err)
	}
	if streamLen != 1 {
		t.Fatalf("stream length = %d, want 1", streamLen)
	}

	messages, err := client.XRange(ctx, "pages", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange returned error: %v", err)
	}
	values := messages[0].Values
	if values["url"] != serverURL {
		t.Fatalf("stream url = %v", values["url"])
	}
	if values["title"] != "Seed Page" {
		t.Fatalf("stream title = %v", values["title"])
	}

	count, err := client.ZCard(ctx, "crawler:frontier").Result()
	if err != nil {
		t.Fatalf("ZCard returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("frontier count = %d, want 1", count)
	}
}
