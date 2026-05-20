package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RedisURL      string
	OpenSearchURL string
	SeedURLs      []string
	MaxDepth      int
	CrawlDelay    time.Duration
	WorkerCount   int
	BatchSize     int
	StreamName    string
	ConsumerGroup string
}

func Load() (Config, error) {
	return loadFromLookup(os.LookupEnv)
}

func loadFromLookup(lookup func(string) (string, bool)) (Config, error) {
	values := make(map[string]string)
	var missing []string
	for _, name := range requiredEnvVars {
		value, ok := lookup(name)
		value = strings.TrimSpace(value)
		if !ok || value == "" {
			missing = append(missing, name)
			continue
		}
		values[name] = value
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	maxDepth, err := parsePositiveInt("MAX_DEPTH", values["MAX_DEPTH"])
	if err != nil {
		return Config{}, err
	}
	workerCount, err := parsePositiveInt("WORKER_COUNT", values["WORKER_COUNT"])
	if err != nil {
		return Config{}, err
	}
	batchSize, err := parsePositiveInt("BATCH_SIZE", values["BATCH_SIZE"])
	if err != nil {
		return Config{}, err
	}
	crawlDelay, err := time.ParseDuration(values["CRAWL_DELAY"])
	if err != nil || crawlDelay <= 0 {
		return Config{}, fmt.Errorf("CRAWL_DELAY must be a positive duration")
	}
	seedURLs, err := parseSeedURLs(values["SEED_URLS"])
	if err != nil {
		return Config{}, err
	}

	return Config{
		RedisURL:      values["REDIS_URL"],
		OpenSearchURL: values["OPENSEARCH_URL"],
		SeedURLs:      seedURLs,
		MaxDepth:      maxDepth,
		CrawlDelay:    crawlDelay,
		WorkerCount:   workerCount,
		BatchSize:     batchSize,
		StreamName:    values["STREAM_NAME"],
		ConsumerGroup: values["CONSUMER_GROUP"],
	}, nil
}

var requiredEnvVars = []string{
	"REDIS_URL",
	"OPENSEARCH_URL",
	"SEED_URLS",
	"MAX_DEPTH",
	"CRAWL_DELAY",
	"WORKER_COUNT",
	"BATCH_SIZE",
	"STREAM_NAME",
	"CONSUMER_GROUP",
}

func parsePositiveInt(name, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func parseSeedURLs(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("SEED_URLS must contain at least one URL")
	}
	return urls, nil
}
