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
	storagev1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/storagev1"
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

	nodeID := cfg.Node.NodeID
	if nodeID == "" {
		nodeID = uuid.NewString()
		log.Info("generated node id", "node_id", nodeID)
	}
	log = log.WithNodeID(nodeID)

	diskStore, err := storage.NewDiskStore(cfg.Storage.DataDir)
	if err != nil {
		log.Error("open disk store", "error", err)
		os.Exit(1)
	}

	port := *grpcPort
	if port == 0 {
		port = 9091
	}
	grpcAddr := fmt.Sprintf("0.0.0.0:%d", port)
	advertiseAddr := cfg.StorageAdvertiseAddr(port)

	totalSpace, _ := diskStore.TotalBytes()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := heartbeat.RegisterWithMaster(ctx, cfg.MasterGRPCAddr(), nodeID, advertiseAddr, totalSpace, log); err != nil {
		log.Error("register with master", "error", err)
		os.Exit(1)
	}

	sender := heartbeat.NewSender(
		nodeID,
		cfg.MasterGRPCAddr(),
		diskStore,
		time.Duration(cfg.Node.HeartbeatInterval)*time.Second,
		log,
	)

	grpcSrv := grpcserver.NewServer(grpcAddr, log)
	storagev1.RegisterStorageServiceServer(grpcSrv.GRPC(), grpcserver.NewStorageGRPCServer(diskStore, nodeID))

	errCh := make(chan error, 3)

	go func() {
		errCh <- sender.Start(ctx)
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
