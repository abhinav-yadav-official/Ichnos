package main

import (
	"fmt"
	"os"

	"github.com/abhinav-yadav-official/Ichnos/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "crawler config error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Ichnos crawler ready: %d workers, %d seed URL(s)\n", cfg.WorkerCount, len(cfg.SeedURLs))
}
