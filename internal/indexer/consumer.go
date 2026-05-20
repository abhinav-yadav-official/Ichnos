package indexer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultFailureHash = "indexer:failures"

type PageIndexer interface {
	BulkIndex(context.Context, []PageDocument) (BulkResult, error)
}

type ConsumerOptions struct {
	Client              *redis.Client
	Indexer             PageIndexer
	StreamName          string
	ConsumerGroup       string
	ConsumerName        string
	BatchSize           int
	BlockTimeout        time.Duration
	MaxDeliveryAttempts int
	FailureHash         string
	Logger              *log.Logger
}

type Consumer struct {
	client              *redis.Client
	indexer             PageIndexer
	streamName          string
	consumerGroup       string
	consumerName        string
	batchSize           int
	blockTimeout        time.Duration
	maxDeliveryAttempts int
	failureHash         string
	logger              *log.Logger
}

func NewConsumer(opts ConsumerOptions) *Consumer {
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	blockTimeout := opts.BlockTimeout
	if blockTimeout <= 0 {
		blockTimeout = 5 * time.Second
	}
	maxAttempts := opts.MaxDeliveryAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	failureHash := opts.FailureHash
	if failureHash == "" {
		failureHash = defaultFailureHash
	}

	return &Consumer{
		client:              opts.Client,
		indexer:             opts.Indexer,
		streamName:          opts.StreamName,
		consumerGroup:       opts.ConsumerGroup,
		consumerName:        opts.ConsumerName,
		batchSize:           batchSize,
		blockTimeout:        blockTimeout,
		maxDeliveryAttempts: maxAttempts,
		failureHash:         failureHash,
		logger:              opts.Logger,
	}
}

func (c *Consumer) EnsureGroup(ctx context.Context) error {
	if err := c.Validate(); err != nil {
		return err
	}

	err := c.client.XGroupCreateMkStream(ctx, c.streamName, c.consumerGroup, "0").Err()
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("create redis stream consumer group: %w", err)
}

func (c *Consumer) Run(ctx context.Context) error {
	if err := c.EnsureGroup(ctx); err != nil {
		return err
	}

	for {
		_, err := c.ProcessOnce(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			c.logf("indexer error: %v", err)
		}
	}
}

func (c *Consumer) ProcessOnce(ctx context.Context) (int, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}

	messages, err := c.readMessages(ctx, "0", 0)
	if err != nil {
		return 0, err
	}
	if len(messages) == 0 {
		messages, err = c.readMessages(ctx, ">", c.blockTimeout)
		if err != nil {
			return 0, err
		}
	}
	if len(messages) == 0 {
		return 0, nil
	}

	docs := make([]PageDocument, 0, len(messages))
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		rawPage, err := RawPageFromStream(message.Values)
		if err != nil {
			if ackErr := c.ack(ctx, message.ID); ackErr != nil {
				return len(ids), fmt.Errorf("ack malformed message %s: %w", message.ID, ackErr)
			}
			c.logf("skipping malformed stream message id=%s: %v", message.ID, err)
			continue
		}
		doc, err := rawPage.Document()
		if err != nil {
			if ackErr := c.ack(ctx, message.ID); ackErr != nil {
				return len(ids), fmt.Errorf("ack unindexable message %s: %w", message.ID, ackErr)
			}
			c.logf("skipping unindexable stream message id=%s: %v", message.ID, err)
			continue
		}
		docs = append(docs, doc)
		ids = append(ids, message.ID)
	}
	if len(docs) == 0 {
		return len(messages), nil
	}

	result, err := c.indexer.BulkIndex(ctx, docs)
	if err != nil {
		if deadLetterErr := c.recordFailures(ctx, ids, err); deadLetterErr != nil {
			return len(messages), deadLetterErr
		}
		return len(messages), fmt.Errorf("bulk index %d docs: %w", len(docs), err)
	}

	if err := c.ack(ctx, ids...); err != nil {
		return len(messages), err
	}
	if len(ids) > 0 {
		c.client.HDel(ctx, c.failureHash, ids...)
	}
	c.logf("indexed docs=%d failed=%d", result.Indexed, result.Failed)
	return len(messages), nil
}

func (c *Consumer) Validate() error {
	if c.client == nil {
		return fmt.Errorf("redis client is required")
	}
	if c.indexer == nil {
		return fmt.Errorf("page indexer is required")
	}
	if c.streamName == "" {
		return fmt.Errorf("stream name is required")
	}
	if c.consumerGroup == "" {
		return fmt.Errorf("consumer group is required")
	}
	if c.consumerName == "" {
		return fmt.Errorf("consumer name is required")
	}
	return nil
}

func (c *Consumer) readMessages(ctx context.Context, id string, block time.Duration) ([]redis.XMessage, error) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.consumerGroup,
		Consumer: c.consumerName,
		Streams:  []string{c.streamName, id},
		Count:    int64(c.batchSize),
		Block:    block,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

func (c *Consumer) recordFailures(ctx context.Context, ids []string, cause error) error {
	for _, id := range ids {
		attempts, err := c.client.HIncrBy(ctx, c.failureHash, id, 1).Result()
		if err != nil {
			return fmt.Errorf("record failed index attempt: %w", err)
		}
		if int(attempts) < c.maxDeliveryAttempts {
			continue
		}
		if err := c.ack(ctx, id); err != nil {
			return fmt.Errorf("dead-letter ack stream message %s: %w", id, err)
		}
		c.client.HDel(ctx, c.failureHash, id)
		c.logf("dead-lettered stream message id=%s attempts=%d cause=%v", id, attempts, cause)
	}
	return nil
}

func (c *Consumer) ack(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.client.XAck(ctx, c.streamName, c.consumerGroup, ids...).Err(); err != nil {
		return fmt.Errorf("ack stream messages: %w", err)
	}
	return nil
}

func (c *Consumer) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}
