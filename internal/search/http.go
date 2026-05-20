package search

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	ichnosmetrics "github.com/abhinav-yadav-official/Ichnos/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Searcher interface {
	Search(context.Context, SearchRequest) (SearchResponse, error)
}

type SearchRequest struct {
	Query  string
	Domain string
	Page   int
	Size   int
}

type SearchResponse struct {
	Hits   []Hit `json:"hits"`
	Total  int   `json:"total"`
	TookMS int   `json:"took_ms"`
	Page   int   `json:"page"`
	Pages  int   `json:"pages"`
}

type Hit struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Snippet   string `json:"snippet"`
	Domain    string `json:"domain"`
	CrawledAt string `json:"crawled_at"`
}

func NewRouter(searcher Searcher) http.Handler {
	return NewRouterWithMetrics(searcher, ichnosmetrics.Default)
}

func NewRouterWithMetrics(searcher Searcher, metrics *ichnosmetrics.Metrics) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/search", searchHandler(searcher, metrics))
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Gatherer, promhttp.HandlerOpts{}))
	return router
}

func searchHandler(searcher Searcher, metrics *ichnosmetrics.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			metrics.SearchLatency.Observe(time.Since(start).Seconds())
		}()
		metrics.SearchRequests.Inc()

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query is required"})
			return
		}
		page := parsePositiveInt(r.URL.Query().Get("page"), 1)

		response, err := searcher.Search(r.Context(), SearchRequest{
			Query:  query,
			Domain: strings.TrimSpace(r.URL.Query().Get("domain")),
			Page:   page,
			Size:   defaultPageSize,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
