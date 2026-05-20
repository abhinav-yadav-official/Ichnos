package indexer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestConsumerIndexesAndAcknowledgesStreamMessages(t *testing.T) {
	ctx := context.Background()
	redisClient := newMiniRedisClient(t)
	indexer := &recordingPageIndexer{}

	consumer := NewConsumer(ConsumerOptions{
		Client:        redisClient,
		Indexer:       indexer,
		StreamName:    "pages",
		ConsumerGroup: "indexers",
		ConsumerName:  "worker-1",
		BatchSize:     100,
		BlockTimeout:  time.Millisecond,
	})

	if err := consumer.EnsureGroup(ctx); err != nil {
		t.Fatalf("EnsureGroup() error = %v", err)
	}
	messageID := addRawPage(t, ctx, redisClient, "pages", "https://example.com/go")

	processed, err := consumer.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("ProcessOnce() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("ProcessOnce() processed = %d, want 1", processed)
	}
	if len(indexer.docs) != 1 {
		t.Fatalf("indexed docs = %d, want 1", len(indexer.docs))
	}
	if indexer.docs[0].URL != "https://example.com/go" || indexer.docs[0].Domain != "example.com" {
		t.Fatalf("indexed doc = %+v, want URL and domain from raw page", indexer.docs[0])
	}

	pending, err := redisClient.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: "pages",
		Group:  "indexers",
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		t.Fatalf("XPendingExt() error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending messages = %+v, want none after ack; message ID was %s", pending, messageID)
	}
}

func TestConsumerDoesNotAcknowledgeFailedBulkIndexUntilDeadLetter(t *testing.T) {
	ctx := context.Background()
	redisClient := newMiniRedisClient(t)
	indexer := &recordingPageIndexer{err: errors.New("bulk failed")}

	consumer := NewConsumer(ConsumerOptions{
		Client:              redisClient,
		Indexer:             indexer,
		StreamName:          "pages",
		ConsumerGroup:       "indexers",
		ConsumerName:        "worker-1",
		BatchSize:           100,
		BlockTimeout:        time.Millisecond,
		MaxDeliveryAttempts: 3,
	})

	if err := consumer.EnsureGroup(ctx); err != nil {
		t.Fatalf("EnsureGroup() error = %v", err)
	}
	addRawPage(t, ctx, redisClient, "pages", "https://example.com/fail")

	for attempt := 1; attempt <= 2; attempt++ {
		processed, err := consumer.ProcessOnce(ctx)
		if err == nil {
			t.Fatalf("attempt %d ProcessOnce() error = nil, want bulk error", attempt)
		}
		if processed != 1 {
			t.Fatalf("attempt %d processed = %d, want 1", attempt, processed)
		}
		pendingCount := pendingMessageCount(t, ctx, redisClient, "pages", "indexers")
		if pendingCount != 1 {
			t.Fatalf("attempt %d pending count = %d, want 1", attempt, pendingCount)
		}
	}

	processed, err := consumer.ProcessOnce(ctx)
	if err == nil {
		t.Fatalf("third ProcessOnce() error = nil, want bulk error")
	}
	if processed != 1 {
		t.Fatalf("third processed = %d, want 1", processed)
	}
	if pending := pendingMessageCount(t, ctx, redisClient, "pages", "indexers"); pending != 0 {
		t.Fatalf("pending count after dead letter = %d, want 0", pending)
	}
}

type recordingPageIndexer struct {
	docs []PageDocument
	err  error
}

func (r *recordingPageIndexer) BulkIndex(_ context.Context, docs []PageDocument) (BulkResult, error) {
	r.docs = append(r.docs, docs...)
	if r.err != nil {
		return BulkResult{Failed: len(docs)}, r.err
	}
	return BulkResult{Indexed: len(docs)}, nil
}

func newMiniRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return client
}

func addRawPage(t *testing.T, ctx context.Context, client *redis.Client, stream, rawURL string) string {
	t.Helper()

	id, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{
			"url":         rawURL,
			"title":       "A Go Page",
			"body":        "Go makes crawlers pleasant",
			"crawled_at":  time.Date(2026, 5, 20, 7, 30, 0, 0, time.UTC).Format(time.RFC3339Nano),
			"word_count":  4,
			"status_code": 200,
		},
	}).Result()
	if err != nil {
		t.Fatalf("XAdd() error = %v", err)
	}
	return id
}

func pendingMessageCount(t *testing.T, ctx context.Context, client *redis.Client, stream, group string) int {
	t.Helper()

	pending, err := client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil {
		t.Fatalf("XPendingExt() error = %v", err)
	}
	return len(pending)
}
