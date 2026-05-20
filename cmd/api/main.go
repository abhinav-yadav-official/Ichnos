package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhinav-yadav-official/Ichnos/internal/config"
	"github.com/abhinav-yadav-official/Ichnos/internal/indexer"
	ichnosmetrics "github.com/abhinav-yadav-official/Ichnos/internal/metrics"
	"github.com/abhinav-yadav-official/Ichnos/internal/search"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "api config error: %v\n", err)
		os.Exit(1)
	}

	openSearchClient, err := indexer.NewClient(cfg.OpenSearchURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "api opensearch config error: %v\n", err)
		os.Exit(1)
	}

	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "api redis config error: %v\n", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go ichnosmetrics.StartQueueDepthReporter(ctx, redisClient, cfg.StreamName, 5*time.Second, ichnosmetrics.Default, log.Default())

	addr := ":8080"
	log.Printf("Ichnos API listening on %s with OpenSearch at %s", addr, cfg.OpenSearchURL)
	if err := http.ListenAndServe(addr, search.NewRouter(search.NewOpenSearchService(openSearchClient))); err != nil {
		fmt.Fprintf(os.Stderr, "api server error: %v\n", err)
		os.Exit(1)
	}
}
