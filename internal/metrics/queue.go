package metrics

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func UpdateQueueDepth(ctx context.Context, client *redis.Client, stream string, metrics *Metrics) error {
	depth, err := client.XLen(ctx, stream).Result()
	if err != nil {
		return err
	}
	metrics.OpenSearchQueueDepth.Set(float64(depth))
	return nil
}

func StartQueueDepthReporter(ctx context.Context, client *redis.Client, stream string, interval time.Duration, metrics *Metrics, logger *log.Logger) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := UpdateQueueDepth(ctx, client, stream, metrics); err != nil && logger != nil {
			logger.Printf("queue depth metrics error: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
