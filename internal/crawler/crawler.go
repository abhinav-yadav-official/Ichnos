package crawler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	ichnosmetrics "github.com/abhinav-yadav-official/Ichnos/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type CrawlerOptions struct {
	Client      *redis.Client
	Frontier    *Frontier
	Seen        *SeenSet
	Politeness  *Politeness
	Fetcher     *Fetcher
	SeedURLs    []string
	MaxDepth    int
	WorkerCount int
	StreamName  string
	Logger      *log.Logger
	Metrics     *ichnosmetrics.Metrics
}

type Crawler struct {
	client      *redis.Client
	frontier    *Frontier
	seen        *SeenSet
	politeness  *Politeness
	fetcher     *Fetcher
	seedURLs    []string
	maxDepth    int
	workerCount int
	streamName  string
	logger      *log.Logger
	metrics     *ichnosmetrics.Metrics
}

func NewCrawler(opts CrawlerOptions) *Crawler {
	workerCount := opts.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}
	return &Crawler{
		client:      opts.Client,
		frontier:    opts.Frontier,
		seen:        opts.Seen,
		politeness:  opts.Politeness,
		fetcher:     opts.Fetcher,
		seedURLs:    opts.SeedURLs,
		maxDepth:    opts.MaxDepth,
		workerCount: workerCount,
		streamName:  opts.StreamName,
		logger:      opts.Logger,
		metrics:     opts.Metrics,
	}
}

func (c *Crawler) Seed(ctx context.Context) error {
	for _, seed := range c.seedURLs {
		normalized, err := NormalizeURL(seed)
		if err != nil {
			return err
		}
		if c.seen.Seen(normalized) {
			continue
		}
		if err := c.frontier.Push(ctx, normalized, 0); err != nil {
			return err
		}
	}
	return nil
}

func (c *Crawler) Run(ctx context.Context) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := c.Seed(ctx); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(ctx)
		}()
	}

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func (c *Crawler) worker(ctx context.Context) {
	for {
		if err := c.CrawlOne(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			if errors.Is(err, ErrFrontierEmpty) {
				if !sleepContext(ctx, 250*time.Millisecond) {
					return
				}
				continue
			}
			if c.metrics != nil {
				c.metrics.CrawlerErrors.WithLabelValues(crawlerErrorType(err)).Inc()
			}
			c.logf("crawl error: %v", err)
		}
	}
}

func (c *Crawler) CrawlOne(ctx context.Context) error {
	item, err := c.frontier.Pop(ctx)
	if err != nil {
		return err
	}

	parsed, err := url.Parse(item.URL)
	if err != nil {
		return err
	}
	host := strings.ToLower(parsed.Hostname())
	if err := c.politeness.Wait(ctx, host); err != nil {
		return err
	}
	allowed, err := c.politeness.Allowed(ctx, item.URL)
	if err != nil {
		return err
	}
	if !allowed {
		c.logf("skipping disallowed URL: %s", item.URL)
		return nil
	}

	page, err := c.fetcher.Fetch(ctx, item.URL)
	if err != nil {
		if errors.Is(err, ErrNonHTML) {
			c.logf("skipping non-HTML URL: %s", item.URL)
			return nil
		}
		return err
	}
	parsedPage, err := NewParser(page.FinalURL).Parse(strings.NewReader(string(page.Body)))
	if err != nil {
		return err
	}
	if err := c.publishPage(ctx, page, parsedPage); err != nil {
		return err
	}
	if c.metrics != nil {
		c.metrics.CrawlerPagesFetched.Inc()
	}
	if item.Depth < c.maxDepth {
		if err := c.pushLinks(ctx, parsedPage.Links, item.Depth+1); err != nil {
			return err
		}
	}
	c.logf("fetched url=%s status=%d links=%d", page.FinalURL, page.StatusCode, len(parsedPage.Links))
	return nil
}

func (c *Crawler) publishPage(ctx context.Context, page FetchedPage, parsed ParsedPage) error {
	_, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: c.streamName,
		Values: map[string]any{
			"url":         page.FinalURL,
			"status_code": page.StatusCode,
			"body":        parsed.Body,
			"title":       parsed.Title,
			"crawled_at":  time.Now().UTC().Format(time.RFC3339Nano),
			"word_count":  wordCount(parsed.Body),
		},
	}).Result()
	return err
}

func (c *Crawler) pushLinks(ctx context.Context, links []string, depth int) error {
	for _, link := range links {
		if c.seen.Seen(link) {
			continue
		}
		if err := c.frontier.Push(ctx, link, depth); err != nil {
			return err
		}
	}
	return nil
}

func (c *Crawler) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

func wordCount(text string) int {
	return len(strings.Fields(text))
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func crawlerErrorType(err error) string {
	if err == nil {
		return "unknown"
	}
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline"
	}
	return "crawl"
}

func (c *Crawler) Validate() error {
	if c.client == nil {
		return fmt.Errorf("redis client is required")
	}
	if c.frontier == nil {
		return fmt.Errorf("frontier is required")
	}
	if c.seen == nil {
		return fmt.Errorf("seen set is required")
	}
	if c.politeness == nil {
		return fmt.Errorf("politeness is required")
	}
	if c.fetcher == nil {
		return fmt.Errorf("fetcher is required")
	}
	if c.streamName == "" {
		return fmt.Errorf("stream name is required")
	}
	return nil
}
