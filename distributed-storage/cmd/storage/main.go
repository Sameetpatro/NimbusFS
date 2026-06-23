package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/heartbeat"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to yaml config file")
	grpcPort := flag.Int("grpc-port", 0, "grpc listen port override; 0 uses env or default 9091")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Observability.LogLevel).WithComponent("storage")

	// auto-generate node id when docker-compose doesn't inject NODE_ID
	nodeID := cfg.Node.NodeID
	if nodeID == "" {
		nodeID = uuid.NewString()
		log.Info("generated node id", "node_id", nodeID)
	}
	log = log.WithNodeID(nodeID)

	localStore, err := storage.NewLocalStore(cfg.Storage.DataDir)
	if err != nil {
		log.Error("open local store", "error", err)
		os.Exit(1)
	}

	port := *grpcPort
	if port == 0 {
		// storage nodes in compose expose 9091-9095; default for bare-metal dev
		port = 9091
	}
	grpcAddr := fmt.Sprintf("0.0.0.0:%d", port)

	grpcSrv := grpcserver.NewServer(grpcAddr, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// heartbeat sender stub until master grpc client is wired in phase 2
	sender := heartbeat.NewSender(
		time.Duration(cfg.Node.HeartbeatInterval)*time.Second,
		nodeID,
		log,
		func(ctx context.Context, id string, used, total int64, chunks int) error {
			_ = ctx
			_ = id
			_ = used
			_ = total
			_ = chunks
			return nil
		},
	)

	errCh := make(chan error, 3)

	go func() {
		errCh <- sender.Run(ctx, func() (int64, int64, int) {
			used, _ := localStore.UsedBytes()
			count, _ := localStore.ChunkCount()
			// total disk space reporting lands in phase 2 with syscall.Statfs
			return used, 0, count
		})
	}()

	go func() {
		metricsAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Observability.MetricsPort)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Info("metrics listening", "addr", metricsAddr)
		errCh <- http.ListenAndServe(metricsAddr, mux)
	}()

	go func() {
		errCh <- grpcSrv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		log.Error("server error", "error", err)
	}

	cancel()
	grpcSrv.Stop()
}
