package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/abhinav-yadav-official/Ichnos/internal/config"
	"github.com/abhinav-yadav-official/Ichnos/internal/indexer"
	"github.com/abhinav-yadav-official/Ichnos/internal/search"
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

	addr := ":8080"
	log.Printf("Ichnos API listening on %s with OpenSearch at %s", addr, cfg.OpenSearchURL)
	if err := http.ListenAndServe(addr, search.NewRouter(search.NewOpenSearchService(openSearchClient))); err != nil {
		fmt.Fprintf(os.Stderr, "api server error: %v\n", err)
		os.Exit(1)
	}
}
