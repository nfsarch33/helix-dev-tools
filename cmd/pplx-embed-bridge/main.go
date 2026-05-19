package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/nfsarch33/helix-dev-tools/internal/pplxbridge"
)

func main() {
	cfg := pplxbridge.DefaultConfig()
	if cfg.APIKey == "" {
		fmt.Fprintf(os.Stderr, "PPLX_API_KEY is required\n")
		os.Exit(1)
	}

	bridge := pplxbridge.NewBridge(cfg)
	log.Printf("pplx-embed-bridge listening on %s (model=%s, dims=%d->%d)\n",
		cfg.ListenAddr, cfg.Model, cfg.Dimensions, cfg.OutputDimensions)
	if err := http.ListenAndServe(cfg.ListenAddr, bridge); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
