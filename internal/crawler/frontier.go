package crawler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/redis/go-redis/v9"
)

var ErrFrontierEmpty = errors.New("frontier is empty")

type Frontier struct {
	client *redis.Client
	key    string
}

type FrontierItem struct {
	URL   string
	Depth int
}

func NewFrontier(client *redis.Client, key string) *Frontier {
	return &Frontier{
		client: client,
		key:    key,
	}
}

func (f *Frontier) Push(ctx context.Context, rawURL string, depth int) error {
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return err
	}
	return f.client.ZAdd(ctx, f.key, redis.Z{
		Score:  float64(depth),
		Member: normalized,
	}).Err()
}

func (f *Frontier) Pop(ctx context.Context) (FrontierItem, error) {
	items, err := f.client.ZPopMin(ctx, f.key, 1).Result()
	if err != nil {
		return FrontierItem{}, err
	}
	if len(items) == 0 {
		return FrontierItem{}, ErrFrontierEmpty
	}
	item, ok := items[0].Member.(string)
	if !ok {
		return FrontierItem{}, fmt.Errorf("frontier member is %T, want string", items[0].Member)
	}
	return FrontierItem{
		URL:   item,
		Depth: int(items[0].Score),
	}, nil
}

func NormalizeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("url must be absolute: %q", rawURL)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = normalizeHost(parsed.Scheme, parsed.Host)
	parsed.Fragment = ""
	parsed.RawQuery = normalizeQuery(parsed.Query())

	return parsed.String(), nil
}

func normalizeHost(scheme, host string) string {
	host = strings.ToLower(host)
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return hostname
	}
	return net.JoinHostPort(hostname, port)
}

func normalizeQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	for key := range values {
		sort.Strings(values[key])
	}
	return values.Encode()
}
