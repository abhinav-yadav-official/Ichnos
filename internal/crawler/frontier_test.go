package crawler

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestNormalizeURLCanonicalizesCrawlURL(t *testing.T) {
	got, err := NormalizeURL("HTTPS://Example.COM:443/path?b=2&a=1#section")
	if err != nil {
		t.Fatalf("NormalizeURL returned error: %v", err)
	}
	want := "https://example.com/path?a=1&b=2"
	if got != want {
		t.Fatalf("NormalizeURL = %q, want %q", got, want)
	}
}

func TestNormalizeURLSortsRepeatedQueryValues(t *testing.T) {
	got, err := NormalizeURL("https://example.com/search?tag=go&q=&tag=crawler")
	if err != nil {
		t.Fatalf("NormalizeURL returned error: %v", err)
	}
	want := "https://example.com/search?q=&tag=crawler&tag=go"
	if got != want {
		t.Fatalf("NormalizeURL = %q, want %q", got, want)
	}
}

func TestFrontierPopsLowestDepthFirst(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	frontier := NewFrontier(client, "crawler:frontier")
	if err := frontier.Push(ctx, "https://example.com/deep", 3); err != nil {
		t.Fatalf("Push deep returned error: %v", err)
	}
	if err := frontier.Push(ctx, "https://example.com/shallow", 1); err != nil {
		t.Fatalf("Push shallow returned error: %v", err)
	}
	if err := frontier.Push(ctx, "https://example.com/middle", 2); err != nil {
		t.Fatalf("Push middle returned error: %v", err)
	}

	first, err := frontier.Pop(ctx)
	if err != nil {
		t.Fatalf("Pop first returned error: %v", err)
	}
	if first.URL != "https://example.com/shallow" || first.Depth != 1 {
		t.Fatalf("first pop = %+v", first)
	}
	second, err := frontier.Pop(ctx)
	if err != nil {
		t.Fatalf("Pop second returned error: %v", err)
	}
	if second.URL != "https://example.com/middle" || second.Depth != 2 {
		t.Fatalf("second pop = %+v", second)
	}
	third, err := frontier.Pop(ctx)
	if err != nil {
		t.Fatalf("Pop third returned error: %v", err)
	}
	if third.URL != "https://example.com/deep" || third.Depth != 3 {
		t.Fatalf("third pop = %+v", third)
	}
}
