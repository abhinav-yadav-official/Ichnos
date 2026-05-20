package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
	"golang.org/x/time/rate"
)

type robotsFetcher func(ctx context.Context, scheme, host string) (string, error)

type Politeness struct {
	userAgent string
	delay     time.Duration
	fetch     robotsFetcher

	mu        sync.Mutex
	limiters  map[string]*rate.Limiter
	robots    map[string]*robotstxt.RobotsData
	robotErrs map[string]error
}

func NewPoliteness(delay time.Duration, userAgent string, fetch robotsFetcher) *Politeness {
	if fetch == nil {
		fetch = fetchRobots
	}
	return &Politeness{
		userAgent: userAgent,
		delay:     delay,
		fetch:     fetch,
		limiters:  make(map[string]*rate.Limiter),
		robots:    make(map[string]*robotstxt.RobotsData),
		robotErrs: make(map[string]error),
	}
}

func (p *Politeness) Allowed(ctx context.Context, rawURL string) (bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return false, fmt.Errorf("url must be absolute: %q", rawURL)
	}
	host := strings.ToLower(parsed.Host)

	data, err := p.robotsFor(ctx, parsed.Scheme, host)
	if err != nil {
		return false, err
	}
	return data.TestAgent(parsed.EscapedPath(), p.userAgent), nil
}

func (p *Politeness) Wait(ctx context.Context, host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("host is required")
	}
	return p.limiterFor(host).Wait(ctx)
}

func (p *Politeness) robotsFor(ctx context.Context, scheme, host string) (*robotstxt.RobotsData, error) {
	p.mu.Lock()
	data, ok := p.robots[host]
	if ok {
		p.mu.Unlock()
		return data, nil
	}
	if err, ok := p.robotErrs[host]; ok {
		p.mu.Unlock()
		return nil, err
	}
	p.mu.Unlock()

	body, err := p.fetch(ctx, scheme, host)
	if err != nil {
		p.mu.Lock()
		p.robotErrs[host] = err
		p.mu.Unlock()
		return nil, err
	}
	data, err = robotstxt.FromString(body)
	if err != nil {
		p.mu.Lock()
		p.robotErrs[host] = err
		p.mu.Unlock()
		return nil, err
	}

	p.mu.Lock()
	p.robots[host] = data
	p.mu.Unlock()
	return data, nil
}

func (p *Politeness) limiterFor(host string) *rate.Limiter {
	p.mu.Lock()
	defer p.mu.Unlock()

	limiter, ok := p.limiters[host]
	if ok {
		return limiter
	}
	limit := rate.Inf
	if p.delay > 0 {
		limit = rate.Every(p.delay)
	}
	limiter = rate.NewLimiter(limit, 1)
	p.limiters[host] = limiter
	return limiter
}

func fetchRobots(ctx context.Context, scheme, host string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+host+"/robots.txt", nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("fetch robots.txt for %s://%s: status %d", scheme, host, res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
