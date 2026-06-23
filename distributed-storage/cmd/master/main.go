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
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// default config path matches docker volume mount in deployments/docker-compose.yml
	configPath := flag.String("config", "configs/config.yaml", "path to yaml config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Observability.LogLevel).WithComponent("master")

	store, err := metadata.NewBoltStore(cfg.Master.DataDir)
	if err != nil {
		log.Error("open metadata store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// gin release mode keeps docker logs quiet; debug is for local dev only
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	api.NewHandlers(store, log).Register(router)

	// metrics on separate mux so we don't mix prometheus middleware with gin routes yet
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// grpc server scaffold; service registration lands in phase 2
	grpcSrv := grpcserver.NewServer(cfg.MasterGRPCAddr(), log)

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

	// block on signal so kubernetes/docker stop sends SIGTERM before we exit
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

	// brief grace period so in-flight http requests can finish
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()
	_ = shutdownCtx
}
