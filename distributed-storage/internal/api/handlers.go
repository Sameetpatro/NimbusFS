package api

import (
	"net/http"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/gin-gonic/gin"
)

// Handlers wires REST routes to metadata store and node registry.
type Handlers struct {
	store    metadata.MetadataStore
	registry *registry.NodeRegistry
	log      *logger.Logger
}

// NewHandlers creates REST handlers with shared dependencies.
func NewHandlers(store metadata.MetadataStore, reg *registry.NodeRegistry, log *logger.Logger) *Handlers {
	return &Handlers{
		store:    store,
		registry: reg,
		log:      log.WithComponent("rest-api"),
	}
}

// Register mounts routes on the gin engine.
func (h *Handlers) Register(r *gin.Engine) {
	r.GET("/health", h.health)
	r.GET("/api/v1/files", h.listFiles)
	r.GET("/api/v1/nodes", h.listNodes)
}

func (h *Handlers) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) listFiles(c *gin.Context) {
	files, err := h.store.ListFiles(c.Request.Context())
	if err != nil {
		h.log.Error("list files failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (h *Handlers) listNodes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"nodes": h.registry.List()})
}
