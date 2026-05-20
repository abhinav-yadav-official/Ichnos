package main

import (
	"fmt"
	"os"

	"github.com/abhinav-yadav-official/Ichnos/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "api config error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Ichnos API ready: OpenSearch at %s\n", cfg.OpenSearchURL)
}
