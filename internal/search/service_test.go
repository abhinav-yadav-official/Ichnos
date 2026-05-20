package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

func TestOpenSearchServiceSearchBuildsQueryAndParsesHits(t *testing.T) {
	var requestPath string
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode search request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"took": 7,
			"timed_out": false,
			"_shards": {"total": 1, "successful": 1, "skipped": 0, "failed": 0},
			"hits": {
				"total": {"value": 1, "relation": "eq"},
				"max_score": 1.0,
				"hits": [
					{
						"_index": "pages-v1",
						"_id": "doc-1",
						"_score": 1.0,
						"_source": {
							"url": "https://go.dev/",
							"title": "Go",
							"body": "The Go programming language",
							"domain": "go.dev",
							"crawled_at": "2026-05-20T07:30:00Z",
							"word_count": 4
						},
						"highlight": {
							"body": ["The <em>Go</em> programming language"]
						}
					}
				]
			}
		}`)
	}))
	defer server.Close()

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{server.URL}},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	service := NewOpenSearchService(client)

	response, err := service.Search(context.Background(), SearchRequest{
		Query:  "golang",
		Domain: "go.dev",
		Page:   2,
		Size:   10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if requestPath != "/pages/_search" {
		t.Fatalf("request path = %q, want /pages/_search", requestPath)
	}
	query := valueAtPath(t, requestBody, "query", "bool", "must", "multi_match")
	if query["query"] != "golang" {
		t.Fatalf("query = %#v, want golang", query["query"])
	}
	filter := valueAtPath(t, requestBody, "query", "bool")["filter"].([]any)
	term := filter[0].(map[string]any)["term"].(map[string]any)
	if term["domain"] != "go.dev" {
		t.Fatalf("domain filter = %#v, want go.dev", term["domain"])
	}

	if response.Total != 1 || response.TookMS != 7 || response.Page != 2 || response.Pages != 1 {
		t.Fatalf("response metadata = %+v, want total=1 took=7 page=2 pages=1", response)
	}
	if len(response.Hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(response.Hits))
	}
	hit := response.Hits[0]
	if hit.URL != "https://go.dev/" || hit.Title != "Go" || hit.Snippet != "The <em>Go</em> programming language" || hit.Domain != "go.dev" {
		t.Fatalf("hit = %+v, want parsed source with highlight snippet", hit)
	}
}

func valueAtPath(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()

	current := root
	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			t.Fatalf("path %v missing object at %q in %#v", path, key, current[key])
		}
		current = next
	}
	return current
}
