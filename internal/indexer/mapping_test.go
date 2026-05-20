package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

func TestPagesIndexMappingMatchesPlan(t *testing.T) {
	body, err := PagesIndexMapping()
	if err != nil {
		t.Fatalf("PagesIndexMapping() error = %v", err)
	}

	var mapping map[string]any
	if err := json.Unmarshal(body, &mapping); err != nil {
		t.Fatalf("mapping is not valid JSON: %v", err)
	}

	assertFieldType(t, mapping, "url", "keyword")
	assertFieldType(t, mapping, "title", "text")
	assertFieldType(t, mapping, "body", "text")
	assertFieldType(t, mapping, "domain", "keyword")
	assertFieldType(t, mapping, "crawled_at", "date")
	assertFieldType(t, mapping, "word_count", "integer")

	title := fieldMapping(t, mapping, "title")
	if boost, ok := title["boost"].(float64); !ok || boost != 3 {
		t.Fatalf("title boost = %#v, want 3", title["boost"])
	}

	defaultSimilarity := valueAt(t, mapping, "settings", "similarity", "default")
	if similarityType, ok := defaultSimilarity["type"].(string); !ok || similarityType != "BM25" {
		t.Fatalf("default similarity type = %#v, want BM25", defaultSimilarity["type"])
	}
}

func TestEnsurePagesIndexCreatesIndexAndAliasOnce(t *testing.T) {
	var indexExists bool
	var aliasExists bool
	var requests []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/pages-v1":
			if !indexExists {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/pages-v1":
			if got := r.Header.Values("Content-Type"); len(got) != 1 || got[0] != "application/json" {
				t.Errorf("Content-Type headers = %#v, want one application/json header", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("create index body is not valid JSON: %v", err)
			}
			assertFieldType(t, body, "title", "text")
			indexExists = true
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"acknowledged":true,"shards_acknowledged":true,"index":"pages-v1"}`)
		case r.Method == http.MethodHead && r.URL.Path == "/pages-v1/_alias/pages":
			if !aliasExists {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/pages-v1/_alias/pages":
			if !indexExists {
				t.Errorf("alias created before index")
			}
			aliasExists = true
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"acknowledged":true}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)

	if err := EnsurePagesIndex(context.Background(), client); err != nil {
		t.Fatalf("first EnsurePagesIndex() error = %v", err)
	}
	if err := EnsurePagesIndex(context.Background(), client); err != nil {
		t.Fatalf("second EnsurePagesIndex() error = %v", err)
	}

	want := []string{
		"HEAD /pages-v1",
		"PUT /pages-v1",
		"HEAD /pages-v1/_alias/pages",
		"PUT /pages-v1/_alias/pages",
		"HEAD /pages-v1",
		"HEAD /pages-v1/_alias/pages",
	}
	if !slices.Equal(requests, want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}

func TestLiveEnsurePagesIndex(t *testing.T) {
	if os.Getenv("ICHNOS_LIVE_TESTS") != "1" {
		t.Skip("set ICHNOS_LIVE_TESTS=1 to run live OpenSearch checks")
	}

	openSearchURL := os.Getenv("OPENSEARCH_URL")
	if openSearchURL == "" {
		t.Fatal("OPENSEARCH_URL is required for live OpenSearch checks")
	}

	client, err := NewClient(openSearchURL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := EnsurePagesIndex(context.Background(), client); err != nil {
		t.Fatalf("first EnsurePagesIndex() error = %v", err)
	}
	if err := EnsurePagesIndex(context.Background(), client); err != nil {
		t.Fatalf("second EnsurePagesIndex() error = %v", err)
	}
}

func newTestClient(t *testing.T, address string) *opensearchapi.Client {
	t.Helper()

	client, err := NewClient(address)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func assertFieldType(t *testing.T, mapping map[string]any, field, want string) {
	t.Helper()

	fieldMap := fieldMapping(t, mapping, field)
	if got, ok := fieldMap["type"].(string); !ok || got != want {
		t.Fatalf("%s type = %#v, want %q", field, fieldMap["type"], want)
	}
}

func fieldMapping(t *testing.T, mapping map[string]any, field string) map[string]any {
	t.Helper()
	return valueAt(t, mapping, "mappings", "properties", field)
}

func valueAt(t *testing.T, value map[string]any, path ...string) map[string]any {
	t.Helper()

	current := value
	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			t.Fatalf("path %q missing object at %q in %#v", path, key, current[key])
		}
		current = next
	}
	return current
}
