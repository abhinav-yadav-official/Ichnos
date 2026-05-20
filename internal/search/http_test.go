package search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterHealth(t *testing.T) {
	server := httptest.NewServer(NewRouter(&fakeSearcher{}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /health: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
}

func TestRouterSearchReturnsJSON(t *testing.T) {
	searcher := &fakeSearcher{
		response: SearchResponse{
			Hits: []Hit{{
				URL:       "https://go.dev/",
				Title:     "Go",
				Snippet:   "The Go programming language",
				Domain:    "go.dev",
				CrawledAt: "2026-05-20T07:30:00Z",
			}},
			Total:  1,
			TookMS: 4,
			Page:   1,
			Pages:  1,
		},
	}
	server := httptest.NewServer(NewRouter(searcher))
	defer server.Close()

	resp, err := http.Get(server.URL + "/search?q=golang")
	if err != nil {
		t.Fatalf("GET /search error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /search status = %d, want 200", resp.StatusCode)
	}
	var body SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /search: %v", err)
	}
	if body.Total != 1 || len(body.Hits) != 1 || body.Hits[0].Title != "Go" {
		t.Fatalf("response = %+v, want one Go hit", body)
	}
	if searcher.request.Query != "golang" || searcher.request.Page != 1 {
		t.Fatalf("search request = %+v, want q golang page 1", searcher.request)
	}
}

func TestRouterSearchParsesDomainAndPage(t *testing.T) {
	searcher := &fakeSearcher{response: SearchResponse{Page: 2}}
	server := httptest.NewServer(NewRouter(searcher))
	defer server.Close()

	resp, err := http.Get(server.URL + "/search?q=golang&domain=github.com&page=2")
	if err != nil {
		t.Fatalf("GET /search error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /search status = %d, want 200", resp.StatusCode)
	}
	if searcher.request.Query != "golang" || searcher.request.Domain != "github.com" || searcher.request.Page != 2 {
		t.Fatalf("search request = %+v, want query/domain/page parsed", searcher.request)
	}
}

func TestRouterSearchRejectsBlankQuery(t *testing.T) {
	server := httptest.NewServer(NewRouter(&fakeSearcher{}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/search?q=%20%20%20")
	if err != nil {
		t.Fatalf("GET /search error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /search blank status = %d, want 400", resp.StatusCode)
	}
}

func TestRouterSearchReturnsBackendErrors(t *testing.T) {
	server := httptest.NewServer(NewRouter(&fakeSearcher{err: errors.New("opensearch down")}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/search?q=golang")
	if err != nil {
		t.Fatalf("GET /search error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("GET /search backend error status = %d, want 500", resp.StatusCode)
	}
}

type fakeSearcher struct {
	request  SearchRequest
	response SearchResponse
	err      error
}

func (f *fakeSearcher) Search(_ context.Context, request SearchRequest) (SearchResponse, error) {
	f.request = request
	return f.response, f.err
}
