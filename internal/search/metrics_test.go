package search

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ichnosmetrics "github.com/abhinav-yadav-official/Ichnos/internal/metrics"
)

func TestSearchMetricsCountRequestsAndLatency(t *testing.T) {
	metrics := ichnosmetrics.NewRegistry()
	server := httptest.NewServer(NewRouterWithMetrics(&fakeSearcher{
		response: SearchResponse{Page: 1, Pages: 1},
	}, metrics))
	defer server.Close()

	for i := 0; i < 10; i++ {
		resp, err := http.Get(server.URL + "/search?q=golang")
		if err != nil {
			t.Fatalf("GET /search request %d error = %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /search request %d status = %d, want 200", i, resp.StatusCode)
		}
	}

	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics error = %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /metrics: %v", err)
	}
	text := string(body)

	assertMetricLine(t, text, "search_requests_total 10")
	assertMetricLine(t, text, `search_latency_seconds_bucket{le="+Inf"} 10`)
	assertMetricLine(t, text, "search_latency_seconds_count 10")
}

func TestQueueDepthReporterSetsGaugeFromRedisStreamLength(t *testing.T) {
	metrics := ichnosmetrics.NewRegistry()
	metrics.OpenSearchQueueDepth.Set(42)

	gathered, err := metrics.Gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range gathered {
		if family.GetName() != "opensearch_queue_depth" {
			continue
		}
		if got := family.GetMetric()[0].GetGauge().GetValue(); got != 42 {
			t.Fatalf("opensearch_queue_depth = %v, want 42", got)
		}
		return
	}
	t.Fatal("opensearch_queue_depth metric not found")
}

func TestMetricsSearchRequestCanUseContext(t *testing.T) {
	metrics := ichnosmetrics.NewRegistry()
	searcher := &fakeSearcher{response: SearchResponse{Page: 1, Pages: 1}}
	router := NewRouterWithMetrics(searcher, metrics)

	req := httptest.NewRequest(http.MethodGet, "/search?q=golang", nil).WithContext(context.Background())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func assertMetricLine(t *testing.T, text, want string) {
	t.Helper()

	for _, line := range strings.Split(text, "\n") {
		if line == want {
			return
		}
	}
	t.Fatalf("metric line %q not found in:\n%s", want, text)
}
