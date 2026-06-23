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

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/api"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/heartbeat"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/observability"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/tlsconfig"
	masterv1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/masterv1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
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
	metrics := observability.NewMetrics()

	var tlsServerCfg *tls.Config
	if cfg.TLS.Enabled {
		tlsServerCfg, err = tlsconfig.LoadOrGenerateTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			log.Error("tls setup failed", "error", err)
			os.Exit(1)
		}
	}

	boltStore, err := metadata.NewBoltStore(cfg.Master.DataDir)
	if err != nil {
		log.Error("open metadata store", "error", err)
		os.Exit(1)
	}

	nodeReg := registry.New()
	if err := loadRegistryFromBolt(context.Background(), nodeReg, boltStore, log); err != nil {
		_ = boltStore.Close()
		log.Error("crash recovery load nodes", "error", err)
		os.Exit(1)
	}

	grpcPool := grpcserver.NewClientPool(log, cfg.TLS.Enabled)
	replMgr := replication.NewManager(nodeReg, boltStore, grpcPool, cfg.Storage.ReplicationFactor, log)

	deadCh := make(chan string, cfg.Storage.ReplicationFactor)
	monitor := heartbeat.NewMonitor(
		nodeReg, boltStore, replMgr, deadCh,
		cfg.Node.HeartbeatInterval, cfg.Node.DeadThreshold, log,
	)

	ctx, cancel := context.WithCancel(context.Background())

	go monitor.Run(ctx)
	go replMgr.ListenDeadNodes(ctx, deadCh)
	go runMetricsRefresh(ctx, metrics, nodeReg, replMgr)

	router := api.NewRouter(cfg, boltStore, nodeReg, replMgr, grpcPool, metrics, log)

	apiServer := &http.Server{Addr: cfg.MasterRESTAddr(), Handler: router}
	metricsAddr := fmt.Sprintf("%s:%d", cfg.Master.Host, cfg.Observability.MetricsPort)
	metricsServer := &http.Server{
		Addr:    metricsAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { promhttp.Handler().ServeHTTP(w, r) }),
	}

	var grpcOpts []grpc.ServerOption
	if tlsServerCfg != nil {
		grpcOpts = grpcserver.ServerOptions(tlsServerCfg)
	}
	grpcSrv := grpcserver.NewServer(cfg.MasterGRPCListenAddr(), log, grpcOpts...)
	masterv1.RegisterMasterServiceServer(grpcSrv.GRPC(), grpcserver.NewMasterGRPCServer(nodeReg, boltStore, boltStore))

	go func() {
		log.Info("rest api listening", "addr", apiServer.Addr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("rest server error", "error", err)
		}
	}()

	go func() {
		log.Info("metrics listening", "addr", metricsServer.Addr)
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

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Error("api shutdown", "error", err)
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics shutdown", "error", err)
	}
	grpcSrv.Stop()
	grpcPool.Close()
	if err := boltStore.Close(); err != nil {
		log.Error("metadata close", "error", err)
	}
}

func runMetricsRefresh(ctx context.Context, metrics *observability.Metrics, reg *registry.NodeRegistry, replMgr *replication.Manager) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics.UpdateNodeMetrics(reg)
			metrics.UpdateReplicationLag(replMgr)
		}
	}
}

func loadRegistryFromBolt(ctx context.Context, reg *registry.NodeRegistry, store *metadata.BoltStore, log *logger.Logger) error {
	nodes, err := store.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("master.loadRegistryFromBolt: %w", err)
	}
	for _, node := range nodes {
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
