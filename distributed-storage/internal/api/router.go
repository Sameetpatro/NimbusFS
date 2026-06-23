package api

import (
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/api/middleware"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/observability"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
	"github.com/gin-gonic/gin"
)

// NewRouter builds the gin engine with security, auth, and api routes.
func NewRouter(
	cfg *config.Config,
	store metadata.MetadataStore,
	reg *registry.NodeRegistry,
	replMgr *replication.Manager,
	pool *grpcserver.ClientPool,
	metrics *observability.Metrics,
	log *logger.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	_ = r.SetTrustedProxies(nil)

	r.Use(gin.Recovery())
	r.Use(middleware.DefaultCORS())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.MaxBodySize(cfg.MaxUploadBytes()))
	r.Use(middleware.NewRateLimitMiddleware(cfg.API.RateLimitRPS, cfg.API.RateLimitBurst))
	r.Use(observability.MetricsMiddleware(metrics))

	h := NewHandlers(cfg, store, reg, replMgr, pool, metrics, log)

	r.GET("/api/v1/health", h.HealthHandler)
	r.GET("/api/v1/cluster/status", h.ClusterStatusHandler)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(cfg.Auth.JWTSecret, cfg.Auth.APIKeyHeader, cfg.Auth.APIKeys))
	{
		v1.POST("/upload", h.UploadHandler)
		v1.GET("/files", h.ListFilesHandler)
		v1.GET("/files/:fileId/download", h.DownloadHandler)
		v1.DELETE("/files/:fileId", h.DeleteFileHandler)
	}

	return r
}
