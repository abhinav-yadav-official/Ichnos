package search

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRouterIndexRendersSearchUI(t *testing.T) {
	server := httptest.NewServer(NewRouter(&fakeSearcher{}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read / body: %v", err)
	}
	text := string(body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	for _, want := range []string{
		`hx-get="/search"`,
		`hx-target="#results"`,
		`hx-trigger="input changed delay:300ms"`,
		`<select`,
		`<option value="go.dev">go.dev</option>`,
		`https://cdn.tailwindcss.com`,
		`https://unpkg.com/htmx.org@1.9.12`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("GET / body missing %q:\n%s", want, text)
		}
	}
}

func TestRouterUsesBasePathForHTMXURLs(t *testing.T) {
	server := httptest.NewServer(NewRouterWithOptions(&fakeSearcher{
		response: SearchResponse{
			Hits: []Hit{{URL: "https://go.dev/", Title: "Go", Snippet: "Go", Domain: "go.dev"}},
			Page: 1, Pages: 2,
		},
	}, RouterOptions{BasePath: "/ichnos"}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/ichnos/")
	if err != nil {
		t.Fatalf("GET /ichnos/ error = %v", err)
	}
	defer resp.Body.Close()
	indexBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /ichnos/ body: %v", err)
	}
	if !strings.Contains(string(indexBody), `hx-get="/ichnos/search"`) {
		t.Fatalf("index body missing base-path search URL:\n%s", indexBody)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/ichnos/search?q=golang", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("HX-Request", "true")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /ichnos/search error = %v", err)
	}
	defer resp.Body.Close()
	partialBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /ichnos/search body: %v", err)
	}
	if !strings.Contains(string(partialBody), `hx-get="/ichnos/search?q=golang&amp;page=2"`) {
		t.Fatalf("partial body missing base-path pagination URL:\n%s", partialBody)
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

func TestRouterSearchReturnsHTMXPartial(t *testing.T) {
	searcher := &fakeSearcher{
		response: SearchResponse{
			Hits: []Hit{{
				URL:       "https://go.dev/",
				Title:     "Go",
				Snippet:   "The <em>Go</em> programming language",
				Domain:    "go.dev",
				CrawledAt: "2026-05-20T07:30:00Z",
			}},
			Total:  1,
			TookMS: 4,
			Page:   1,
			Pages:  2,
		},
	}
	server := httptest.NewServer(NewRouter(searcher))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/search?q=golang", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("HX-Request", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /search htmx error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /search htmx body: %v", err)
	}
	text := string(body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /search htmx status = %d, want 200", resp.StatusCode)
	}
	for _, want := range []string{
		`href="https://go.dev/"`,
		`Go`,
		`The <mark>Go</mark> programming language`,
		`go.dev`,
		`hx-get="/search?q=golang&amp;page=2"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("GET /search htmx body missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "<!doctype html>") {
		t.Fatalf("GET /search htmx returned full page, want partial:\n%s", text)
	}
}

func TestRouterSearchHTMXNoResults(t *testing.T) {
	searcher := &fakeSearcher{response: SearchResponse{Page: 1}}
	server := httptest.NewServer(NewRouter(searcher))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/search?q=missing", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("HX-Request", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /search htmx error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /search htmx body: %v", err)
	}
	if !strings.Contains(string(body), "No results") {
		t.Fatalf("GET /search htmx no results missing empty state:\n%s", body)
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
