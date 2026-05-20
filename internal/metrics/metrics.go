package metrics

import "github.com/prometheus/client_golang/prometheus"

var Default = New(prometheus.DefaultRegisterer, prometheus.DefaultGatherer)

type Metrics struct {
	CrawlerPagesFetched  prometheus.Counter
	CrawlerErrors        *prometheus.CounterVec
	IndexerDocsIndexed   prometheus.Counter
	SearchRequests       prometheus.Counter
	SearchLatency        prometheus.Histogram
	OpenSearchQueueDepth prometheus.Gauge
	Gatherer             prometheus.Gatherer
}

func NewRegistry() *Metrics {
	registry := prometheus.NewRegistry()
	return New(registry, registry)
}

func New(registerer prometheus.Registerer, gatherer prometheus.Gatherer) *Metrics {
	m := &Metrics{
		CrawlerPagesFetched: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "crawler_pages_fetched_total",
			Help: "Total number of pages successfully fetched by the crawler.",
		}),
		CrawlerErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "crawler_errors_total",
			Help: "Total number of crawler errors by type.",
		}, []string{"type"}),
		IndexerDocsIndexed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "indexer_docs_indexed_total",
			Help: "Total number of documents successfully indexed into OpenSearch.",
		}),
		SearchRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "search_requests_total",
			Help: "Total number of search requests.",
		}),
		SearchLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "search_latency_seconds",
			Help:    "Search request latency in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1},
		}),
		OpenSearchQueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "opensearch_queue_depth",
			Help: "Current Redis stream length for pages waiting near the OpenSearch indexing path.",
		}),
		Gatherer: gatherer,
	}

	registerer.MustRegister(
		m.CrawlerPagesFetched,
		m.CrawlerErrors,
		m.IndexerDocsIndexed,
		m.SearchRequests,
		m.SearchLatency,
		m.OpenSearchQueueDepth,
	)
	return m
}
