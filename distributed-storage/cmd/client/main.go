package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to yaml config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Observability.LogLevel).WithComponent("client")

	// phase 1 cli is a stub; upload/download subcommands arrive in phase 2
	log.Info("nimbusfs client scaffold ready",
		"master", cfg.MasterRESTAddr(),
		"chunk_size_mb", cfg.Storage.ChunkSizeMB,
		"replication_factor", cfg.Storage.ReplicationFactor,
	)
	fmt.Println("nimbusfs client — phase 1 scaffold (use phase 2 for upload/download)")
}
