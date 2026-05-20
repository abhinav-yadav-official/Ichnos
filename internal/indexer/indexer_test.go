package indexer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildBulkBodyUsesAliasStableIDAndDomain(t *testing.T) {
	crawledAt := time.Date(2026, 5, 20, 7, 30, 0, 0, time.UTC)
	page := RawPage{
		URL:       "https://example.com/articles/go?ref=home",
		Title:     "Go article",
		Body:      "Go makes crawlers pleasant",
		CrawledAt: crawledAt,
		WordCount: 4,
	}

	doc, err := page.Document()
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}
	body, err := BuildBulkBody([]PageDocument{doc})
	if err != nil {
		t.Fatalf("BuildBulkBody() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 2 {
		t.Fatalf("bulk body has %d lines, want 2: %s", len(lines), body)
	}

	var action map[string]map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &action); err != nil {
		t.Fatalf("action line is not JSON: %v", err)
	}
	if action["index"]["_index"] != PagesAliasName {
		t.Fatalf("bulk _index = %q, want %q", action["index"]["_index"], PagesAliasName)
	}
	if action["index"]["_id"] != expectedDocumentID(page.URL) {
		t.Fatalf("bulk _id = %q, want stable URL hash", action["index"]["_id"])
	}

	var indexed PageDocument
	if err := json.Unmarshal([]byte(lines[1]), &indexed); err != nil {
		t.Fatalf("document line is not JSON: %v", err)
	}
	if indexed.Domain != "example.com" {
		t.Fatalf("domain = %q, want example.com", indexed.Domain)
	}
	if indexed.CrawledAt != crawledAt.Format(time.RFC3339Nano) {
		t.Fatalf("crawled_at = %q, want %q", indexed.CrawledAt, crawledAt.Format(time.RFC3339Nano))
	}
}

func TestBulkIndexerPostsBulkRequestAndReportsItemFailures(t *testing.T) {
	var method string
	var path string
	var lines []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		scanner := bufio.NewScanner(r.Body)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			t.Errorf("read bulk request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"errors": true,
			"items": [
				{"index": {"_id": "ok", "status": 201}},
				{"index": {"_id": "bad", "status": 400, "error": {"type": "mapper_parsing_exception", "reason": "bad field"}}}
			]
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	bulkIndexer := NewBulkIndexer(client)

	result, err := bulkIndexer.BulkIndex(context.Background(), []PageDocument{
		{ID: "ok", URL: "https://example.com/ok", Title: "ok", Body: "ok", Domain: "example.com", CrawledAt: time.Now().UTC().Format(time.RFC3339Nano), WordCount: 1},
		{ID: "bad", URL: "https://example.com/bad", Title: "bad", Body: "bad", Domain: "example.com", CrawledAt: time.Now().UTC().Format(time.RFC3339Nano), WordCount: 1},
	})
	if err == nil {
		t.Fatalf("BulkIndex() error = nil, want item failure error")
	}
	if result.Indexed != 1 || result.Failed != 1 {
		t.Fatalf("BulkIndex() result = %+v, want 1 indexed and 1 failed", result)
	}
	if len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Reason, "bad field") {
		t.Fatalf("BulkIndex() failures = %+v, want bad field failure", result.Failures)
	}
	if method != http.MethodPost || path != "/_bulk" {
		t.Fatalf("bulk request = %s %s, want POST /_bulk", method, path)
	}
	if len(lines) != 4 {
		t.Fatalf("bulk request lines = %d, want 4", len(lines))
	}
}

func expectedDocumentID(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}
