package metrics

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

func TestUpdateQueueDepthSetsGaugeFromRedisStreamLength(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})

	for i := 0; i < 3; i++ {
		if err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: "pages",
			Values: map[string]any{"url": "https://example.com"},
		}).Err(); err != nil {
			t.Fatalf("XAdd() error = %v", err)
		}
	}

	metrics := NewRegistry()
	if err := UpdateQueueDepth(ctx, client, "pages", metrics); err != nil {
		t.Fatalf("UpdateQueueDepth() error = %v", err)
	}
	if got := testutil.ToFloat64(metrics.OpenSearchQueueDepth); got != 3 {
		t.Fatalf("opensearch_queue_depth = %v, want 3", got)
	}
}
