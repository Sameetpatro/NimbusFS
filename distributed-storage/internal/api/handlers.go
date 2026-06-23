package api

import (
	"net/http"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/gin-gonic/gin"
)

// Handlers wires REST routes to metadata store; upload/download land in phase 2.
type Handlers struct {
	store metadata.Store
	log   *logger.Logger
}

// NewHandlers creates REST handlers with shared dependencies.
func NewHandlers(store metadata.Store, log *logger.Logger) *Handlers {
	return &Handlers{
		store: store,
		log:   log.WithComponent("rest-api"),
	}
}

// Register mounts routes on the gin engine.
func (h *Handlers) Register(r *gin.Engine) {
	// health is unauthenticated so k8s liveness probes work without secrets
	r.GET("/health", h.health)
	r.GET("/api/v1/files", h.listFiles)
}

// health returns 200 when the process is up; deeper checks come in phase 3.
func (h *Handlers) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// listFiles returns metadata for all files known to the master.
func (h *Handlers) listFiles(c *gin.Context) {
	files, err := h.store.ListFiles(c.Request.Context())
	if err != nil {
		h.log.Error("list files failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}
