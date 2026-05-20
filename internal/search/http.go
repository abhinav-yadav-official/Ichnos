package search

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	ichnosmetrics "github.com/abhinav-yadav-official/Ichnos/internal/metrics"
	ichnostemplates "github.com/abhinav-yadav-official/Ichnos/templates"
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

type RouterOptions struct {
	Metrics  *ichnosmetrics.Metrics
	BasePath string
}

func NewRouter(searcher Searcher) http.Handler {
	return NewRouterWithMetrics(searcher, ichnosmetrics.Default)
}

func NewRouterWithMetrics(searcher Searcher, metrics *ichnosmetrics.Metrics) http.Handler {
	return NewRouterWithOptions(searcher, RouterOptions{Metrics: metrics})
}

func NewRouterWithOptions(searcher Searcher, opts RouterOptions) http.Handler {
	metrics := opts.Metrics
	if metrics == nil {
		metrics = ichnosmetrics.Default
	}
	basePath := normalizeBasePath(opts.BasePath)
	tmpl := template.Must(ichnostemplates.Parse())
	router := chi.NewRouter()
	registerSearchRoutes(router, searcher, metrics, tmpl, basePath)
	if basePath != "" {
		router.Mount(basePath, searchRoutes(searcher, metrics, tmpl, basePath))
	}
	return router
}

func registerSearchRoutes(router chi.Router, searcher Searcher, metrics *ichnosmetrics.Metrics, tmpl *template.Template, basePath string) {
	router.Mount("/", searchRoutes(searcher, metrics, tmpl, basePath))
}

func searchRoutes(searcher Searcher, metrics *ichnosmetrics.Metrics, tmpl *template.Template, basePath string) http.Handler {
	router := chi.NewRouter()
	router.Get("/", indexHandler(tmpl, basePath))
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/search", searchHandler(searcher, metrics, tmpl, basePath))
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Gatherer, promhttp.HandlerOpts{}))
	return router
}

type templateData struct {
	Query    string
	Domain   string
	Response SearchResponse
	BasePath string
}

func indexHandler(tmpl *template.Template, basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, http.StatusOK)
		tmpl.ExecuteTemplate(w, "index.html", templateData{BasePath: basePath})
	}
}

func searchHandler(searcher Searcher, metrics *ichnosmetrics.Metrics, tmpl *template.Template, basePath string) http.HandlerFunc {
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
		if wantsHTML(r) {
			writeHTML(w, http.StatusOK)
			tmpl.ExecuteTemplate(w, "results.html", templateData{
				Query:    query,
				Domain:   strings.TrimSpace(r.URL.Query().Get("domain")),
				Response: response,
				BasePath: basePath,
			})
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

func writeHTML(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
}

func wantsHTML(r *http.Request) bool {
	if r.Header.Get("HX-Request") == "true" {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
}

func normalizeBasePath(value string) string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return ""
	}
	return "/" + trimmed
}
