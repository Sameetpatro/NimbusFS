package main

import (
	"context"
	"crypto/tls"
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
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/observability"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
	"google.golang.org/grpc"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/tlsconfig"
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

	tlsCfg, err := loadTLS(cfg)
	if err != nil {
		log.Error("tls setup failed", "error", err)
		os.Exit(1)
	}

	diskStore, err := storage.NewDiskStore(cfg.Storage.DataDir)
	if err != nil {
		log.Error("open disk store", "error", err)
		os.Exit(1)
	}
	observability.RegisterStorageNodeCollector(nodeID, diskStore)

	port := *grpcPort
	if port == 0 {
		port = 9091
	}
	grpcAddr := fmt.Sprintf("0.0.0.0:%d", port)
	advertiseAddr := cfg.StorageAdvertiseAddr(port)
	totalSpace, _ := diskStore.TotalBytes()

	ctx, cancel := context.WithCancel(context.Background())

	if err := heartbeat.RegisterWithMaster(ctx, cfg.MasterGRPCAddr(), nodeID, advertiseAddr, totalSpace, log, cfg.TLS.Enabled); err != nil {
		cancel()
		log.Error("register with master", "error", err)
		os.Exit(1)
	}

	sender := heartbeat.NewSender(nodeID, cfg.MasterGRPCAddr(), diskStore,
		time.Duration(cfg.Node.HeartbeatInterval)*time.Second, log, cfg.TLS.Enabled)

	var grpcOpts []grpc.ServerOption
	if tlsCfg != nil {
		grpcOpts = grpcserver.ServerOptions(tlsCfg)
	}
	grpcSrv := grpcserver.NewServer(grpcAddr, log, grpcOpts...)
	storagev1.RegisterStorageServiceServer(grpcSrv.GRPC(), grpcserver.NewStorageGRPCServer(diskStore, nodeID, cfg.TLS.Enabled))

	metricsAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Observability.MetricsPort)
	metricsServer := &http.Server{
		Addr: metricsAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			promhttp.Handler().ServeHTTP(w, r)
		}),
	}

	go func() {
		if err := sender.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("heartbeat sender stopped", "error", err)
		}
	}()

	go func() {
		log.Info("metrics listening", "addr", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server error", "error", err)
		}
	}()

	go func() {
		if err := grpcSrv.ListenAndServe(); err != nil {
			log.Error("grpc server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutdown signal received")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics shutdown", "error", err)
	}
	grpcSrv.Stop()
}

func loadTLS(cfg *config.Config) (*tls.Config, error) {
	if !cfg.TLS.Enabled {
		return nil, nil
	}
	return tlsconfig.LoadOrGenerateTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
}
