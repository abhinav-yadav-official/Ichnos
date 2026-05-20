package docker_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestGrafanaProvisioningAndDashboardConfig(t *testing.T) {
	compose := readFile(t, "../docker-compose.yml")
	for _, want := range []string{
		"./docker/grafana/provisioning:/etc/grafana/provisioning:ro",
		"./docker/grafana/dashboards:/var/lib/grafana/dashboards:ro",
		"./docker/alerts.yml:/etc/prometheus/alerts.yml:ro",
		"host.docker.internal:host-gateway",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("docker-compose.yml missing %q", want)
		}
	}

	datasource := readFile(t, "grafana/provisioning/datasources/prometheus.yml")
	for _, want := range []string{"name: Prometheus", "url: http://prometheus:9090", "isDefault: true"} {
		if !strings.Contains(datasource, want) {
			t.Fatalf("datasource provisioning missing %q", want)
		}
	}

	provider := readFile(t, "grafana/provisioning/dashboards/dashboards.yml")
	for _, want := range []string{"path: /var/lib/grafana/dashboards", "disableDeletion: false"} {
		if !strings.Contains(provider, want) {
			t.Fatalf("dashboard provisioning missing %q", want)
		}
	}

	var dashboard struct {
		Title  string `json:"title"`
		Panels []struct {
			Title   string `json:"title"`
			Targets []struct {
				Expr string `json:"expr"`
			} `json:"targets"`
		} `json:"panels"`
	}
	if err := json.Unmarshal([]byte(readFile(t, "grafana/dashboards/crawler.json")), &dashboard); err != nil {
		t.Fatalf("dashboard JSON invalid: %v", err)
	}
	if dashboard.Title != "Ichnos crawler and search" {
		t.Fatalf("dashboard title = %q", dashboard.Title)
	}
	wantPanels := map[string]string{
		"Crawl rate":         "rate(crawler_pages_fetched_total[1m]) * 60",
		"Index lag":          "opensearch_queue_depth",
		"Search p95 latency": "histogram_quantile(0.95, sum(rate(search_latency_seconds_bucket[5m])) by (le))",
		"Error rate by type": "rate(crawler_errors_total[5m])",
		"Total docs indexed": "indexer_docs_indexed_total",
		"Active goroutines":  "go_goroutines",
	}
	for title, expr := range wantPanels {
		if !dashboardHasPanel(dashboard.Panels, title, expr) {
			t.Fatalf("dashboard missing panel %q with expr %q", title, expr)
		}
	}
}

func TestPrometheusScrapesIchnosTargetsAndLoadsAlerts(t *testing.T) {
	prometheus := readFile(t, "prometheus.yml")
	for _, want := range []string{
		"rule_files:",
		"alerts.yml",
		"job_name: ichnos-api",
		"host.docker.internal:8080",
		"job_name: ichnos-crawler",
		"host.docker.internal:6060",
	} {
		if !strings.Contains(prometheus, want) {
			t.Fatalf("prometheus.yml missing %q", want)
		}
	}

	alerts := readFile(t, "alerts.yml")
	for _, want := range []string{
		"IchnosCrawlerHighErrorRate",
		"crawler_errors_total",
		"crawler_pages_fetched_total",
		"severity: warning",
	} {
		if !strings.Contains(alerts, want) {
			t.Fatalf("alerts.yml missing %q", want)
		}
	}
}

func TestProductionComposeTargetsIchnosSubpath(t *testing.T) {
	compose := readFile(t, "../docker-compose.prod.yml")
	for _, want := range []string{
		"BASE_PATH=/ichnos",
		"GF_SERVER_ROOT_URL=https://abhiyadav.in/ichnos/grafana/",
		"GF_SERVER_SERVE_FROM_SUB_PATH=true",
		"./docker/prometheus.prod.yml:/etc/prometheus/prometheus.yml:ro",
		"./docker/grafana/provisioning:/etc/grafana/provisioning:ro",
		"./docker/grafana/dashboards:/var/lib/grafana/dashboards:ro",
		"./docker/nginx/nginx.conf:/etc/nginx/conf.d/default.conf:ro",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("docker-compose.prod.yml missing %q", want)
		}
	}

	prometheus := readFile(t, "prometheus.prod.yml")
	for _, want := range []string{
		"api:8080",
		"crawler:6060",
		"alerts.yml",
	} {
		if !strings.Contains(prometheus, want) {
			t.Fatalf("prometheus.prod.yml missing %q", want)
		}
	}
}

func TestNginxConfigRoutesIchnosSubpath(t *testing.T) {
	nginx := readFile(t, "nginx/nginx.conf")
	for _, want := range []string{
		"server_name abhiyadav.in;",
		"return 301 /ichnos/;",
		"location /ichnos/",
		"proxy_pass http://api:8080/;",
		"proxy_set_header X-Forwarded-Prefix /ichnos;",
		"location /ichnos/grafana/",
		"proxy_pass http://grafana:3000/;",
		"/etc/letsencrypt/live/abhiyadav.in/fullchain.pem",
		"/etc/letsencrypt/live/abhiyadav.in/privkey.pem",
	} {
		if !strings.Contains(nginx, want) {
			t.Fatalf("nginx.conf missing %q", want)
		}
	}
}

func dashboardHasPanel(panels []struct {
	Title   string `json:"title"`
	Targets []struct {
		Expr string `json:"expr"`
	} `json:"targets"`
}, title, expr string) bool {
	for _, panel := range panels {
		if panel.Title != title {
			continue
		}
		for _, target := range panel.Targets {
			if target.Expr == expr {
				return true
			}
		}
	}
	return false
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
