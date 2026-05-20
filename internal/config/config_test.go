package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromLookupParsesConfig(t *testing.T) {
	cfg, err := loadFromLookup(mapLookup(map[string]string{
		"REDIS_URL":      "redis://localhost:6379",
		"OPENSEARCH_URL": "http://localhost:9201",
		"SEED_URLS":      "https://example.com, https://news.ycombinator.com",
		"MAX_DEPTH":      "3",
		"CRAWL_DELAY":    "1500ms",
		"WORKER_COUNT":   "5",
		"BATCH_SIZE":     "100",
		"STREAM_NAME":    "pages",
		"CONSUMER_GROUP": "indexers",
	}))
	if err != nil {
		t.Fatalf("loadFromLookup returned error: %v", err)
	}

	if cfg.RedisURL != "redis://localhost:6379" {
		t.Fatalf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.OpenSearchURL != "http://localhost:9201" {
		t.Fatalf("OpenSearchURL = %q", cfg.OpenSearchURL)
	}
	if got := strings.Join(cfg.SeedURLs, ","); got != "https://example.com,https://news.ycombinator.com" {
		t.Fatalf("SeedURLs = %q", got)
	}
	if cfg.MaxDepth != 3 {
		t.Fatalf("MaxDepth = %d", cfg.MaxDepth)
	}
	if cfg.CrawlDelay != 1500*time.Millisecond {
		t.Fatalf("CrawlDelay = %s", cfg.CrawlDelay)
	}
	if cfg.WorkerCount != 5 {
		t.Fatalf("WorkerCount = %d", cfg.WorkerCount)
	}
	if cfg.BatchSize != 100 {
		t.Fatalf("BatchSize = %d", cfg.BatchSize)
	}
	if cfg.StreamName != "pages" {
		t.Fatalf("StreamName = %q", cfg.StreamName)
	}
	if cfg.ConsumerGroup != "indexers" {
		t.Fatalf("ConsumerGroup = %q", cfg.ConsumerGroup)
	}
}

func TestLoadFromLookupRequiresAllEnvVars(t *testing.T) {
	_, err := loadFromLookup(mapLookup(map[string]string{
		"REDIS_URL": "redis://localhost:6379",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, name := range []string{
		"OPENSEARCH_URL",
		"SEED_URLS",
		"MAX_DEPTH",
		"CRAWL_DELAY",
		"WORKER_COUNT",
		"BATCH_SIZE",
		"STREAM_NAME",
		"CONSUMER_GROUP",
	} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error %q does not mention %s", err.Error(), name)
		}
	}
}

func TestLoadFromLookupRejectsInvalidTypes(t *testing.T) {
	values := validEnv()
	values["MAX_DEPTH"] = "zero"

	_, err := loadFromLookup(mapLookup(values))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "MAX_DEPTH") {
		t.Fatalf("error %q does not mention MAX_DEPTH", err.Error())
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"REDIS_URL":      "redis://localhost:6379",
		"OPENSEARCH_URL": "http://localhost:9201",
		"SEED_URLS":      "https://example.com",
		"MAX_DEPTH":      "3",
		"CRAWL_DELAY":    "1s",
		"WORKER_COUNT":   "5",
		"BATCH_SIZE":     "100",
		"STREAM_NAME":    "pages",
		"CONSUMER_GROUP": "indexers",
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
