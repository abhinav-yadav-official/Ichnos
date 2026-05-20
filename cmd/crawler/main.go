package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhinav-yadav-official/Ichnos/internal/config"
	"github.com/abhinav-yadav-official/Ichnos/internal/crawler"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "crawler config error: %v\n", err)
		os.Exit(1)
	}

	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crawler redis config error: %v\n", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	go func() {
		log.Printf("pprof listening on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Printf("pprof server stopped: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := crawler.NewCrawler(crawler.CrawlerOptions{
		Client:      redisClient,
		Frontier:    crawler.NewFrontier(redisClient, "crawler:frontier"),
		Seen:        crawler.NewSeenSet(10_000_000, 0.0001),
		Politeness:  crawler.NewPoliteness(cfg.CrawlDelay, "CrawlerBot/1.0", nil),
		Fetcher:     crawler.NewFetcher("CrawlerBot/1.0", 10*time.Second),
		SeedURLs:    cfg.SeedURLs,
		MaxDepth:    cfg.MaxDepth,
		WorkerCount: cfg.WorkerCount,
		StreamName:  cfg.StreamName,
		Logger:      log.Default(),
	})
	log.Printf("starting crawler workers=%d seeds=%d max_depth=%d", cfg.WorkerCount, len(cfg.SeedURLs), cfg.MaxDepth)
	if err := c.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "crawler run error: %v\n", err)
		os.Exit(1)
	}
}
