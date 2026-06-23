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

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/api"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/heartbeat"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
	masterv1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/masterv1"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to yaml config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Observability.LogLevel).WithComponent("master")

	boltStore, err := metadata.NewBoltStore(cfg.Master.DataDir)
	if err != nil {
		log.Error("open metadata store", "error", err)
		os.Exit(1)
	}
	defer boltStore.Close()

	nodeReg := registry.New()
	if err := loadRegistryFromBolt(context.Background(), nodeReg, boltStore, log); err != nil {
		log.Error("crash recovery load nodes", "error", err)
		os.Exit(1)
	}

	grpcPool := grpcserver.NewClientPool(log)
	defer grpcPool.Close()

	replMgr := replication.NewManager(nodeReg, boltStore, grpcPool, cfg.Storage.ReplicationFactor, log)

	deadCh := make(chan string, cfg.Storage.ReplicationFactor)
	monitor := heartbeat.NewMonitor(
		nodeReg,
		boltStore,
		replMgr,
		deadCh,
		cfg.Node.HeartbeatInterval,
		cfg.Node.DeadThreshold,
		log,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go monitor.Run(ctx)
	go replMgr.ListenDeadNodes(ctx, deadCh)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	api.NewHandlers(boltStore, nodeReg, log).Register(router)

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	grpcSrv := grpcserver.NewServer(cfg.MasterGRPCListenAddr(), log)
	masterv1.RegisterMasterServiceServer(grpcSrv.GRPC(), grpcserver.NewMasterGRPCServer(nodeReg, boltStore, boltStore))

	errCh := make(chan error, 3)

	go func() {
		log.Info("rest api listening", "addr", cfg.MasterRESTAddr())
		errCh <- http.ListenAndServe(cfg.MasterRESTAddr(), router)
	}()

	go func() {
		metricsAddr := fmt.Sprintf("%s:%d", cfg.Master.Host, cfg.Observability.MetricsPort)
		log.Info("metrics listening", "addr", metricsAddr)
		errCh <- http.ListenAndServe(metricsAddr, metricsMux)
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = shutdownCtx
}

// loadRegistryFromBolt hydrates in-memory registry from boltdb for crash recovery on startup.
func loadRegistryFromBolt(ctx context.Context, reg *registry.NodeRegistry, store *metadata.BoltStore, log *logger.Logger) error {
	nodes, err := store.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("master.loadRegistryFromBolt: %w", err)
	}

	for _, node := range nodes {
		// mark recovered nodes suspect until a fresh heartbeat proves they're alive
		node.Status = domain.NodeStatusSuspect
		reg.Register(node)
		log.Info("recovered node from boltdb", "node_id", node.NodeID, "address", node.Address)
	}

	files, err := store.ListFiles(ctx)
	if err != nil {
		return fmt.Errorf("master.loadRegistryFromBolt: list files: %w", err)
	}
	log.Info("crash recovery complete", "nodes", len(nodes), "files", len(files))
	return nil
}
