package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/apperrors"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/chunking"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/observability"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
	"github.com/gin-gonic/gin"
)

// Handlers wires REST routes; handlers stay thin and delegate to services.
type Handlers struct {
	upload   *UploadService
	download *DownloadService
	delete   *DeleteService
	store    metadata.MetadataStore
	registry *registry.NodeRegistry
	replMgr  *replication.Manager
	metrics  *observability.Metrics
	cfg      *config.Config
	log      *logger.Logger
}

// NewHandlers creates REST handlers with shared dependencies.
func NewHandlers(
	cfg *config.Config,
	store metadata.MetadataStore,
	reg *registry.NodeRegistry,
	replMgr *replication.Manager,
	pool *grpcserver.ClientPool,
	metrics *observability.Metrics,
	log *logger.Logger,
) *Handlers {
	chunker := chunking.New(cfg.ChunkSizeBytes())
	return &Handlers{
		upload:   NewUploadService(chunker, replMgr, store, pool, cfg.Storage.ReplicationFactor, cfg.ChunkSizeBytes()),
		download: NewDownloadService(store, pool, replMgr),
		delete:   NewDeleteService(store, replMgr),
		store:    store,
		registry: reg,
		replMgr:  replMgr,
		metrics:  metrics,
		cfg:      cfg,
		log:      log.WithComponent("rest-api"),
	}
}

// UploadHandler parses the multipart form, delegates chunking and storage to the upload service.
func (h *Handlers) UploadHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file in request"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot open uploaded file"})
		return
	}
	defer f.Close()

	result, err := h.upload.Upload(c.Request.Context(), file.Filename, f)
	if err != nil {
		if IsInsufficientStorage(err) {
			c.JSON(507, gin.H{"error": "insufficient storage"})
			return
		}
		h.log.Error("upload failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}

	h.metrics.ObserveUpload(result.Size)
	c.JSON(http.StatusCreated, result)
}

// DownloadHandler streams file bytes back to the client by fetching and reassembling chunks.
func (h *Handlers) DownloadHandler(c *gin.Context) {
	fileID := c.Param("fileId")

	meta, err := h.store.GetFile(c.Request.Context(), fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, meta.FileName))
	c.Header("Content-Type", "application/octet-stream")
	c.Status(http.StatusPartialContent)

	// write directly to response writer — avoids gin.Stream CloseNotifier requirement in httptest
	written, err := h.download.Stream(c.Request.Context(), fileID, c.Writer)
	if err != nil {
		h.log.Error("download failed", "file_id", fileID, "error", err)
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "download failed"})
		}
		return
	}
	h.metrics.ObserveDownload(written.Size)
}

// DeleteFileHandler deletes metadata and chunk replicas.
func (h *Handlers) DeleteFileHandler(c *gin.Context) {
	fileID := c.Param("fileId")
	if err := h.delete.Delete(c.Request.Context(), fileID); err != nil {
		if errors.Is(err, apperrors.ErrFileNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		h.log.Error("delete failed", "file_id", fileID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	h.metrics.ObserveDelete()
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListFilesHandler returns paginated file metadata.
func (h *Handlers) ListFilesHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	files, err := h.store.ListFiles(c.Request.Context())
	if err != nil {
		h.log.Error("list files failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files"})
		return
	}

	total := len(files)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files[start:end],
		"total": total,
		"page":  page,
	})
}

// ClusterStatusHandler returns node health and storage totals.
func (h *Handlers) ClusterStatusHandler(c *gin.Context) {
	nodes := h.registry.List()
	var totalStorage, usedStorage int64
	alive := 0
	for _, n := range nodes {
		totalStorage += n.TotalSpace
		usedStorage += n.UsedSpace
		if n.Status.String() == "alive" {
			alive++
		}
	}

	h.metrics.UpdateNodeMetrics(h.registry)
	h.metrics.UpdateReplicationLag(h.replMgr)

	c.JSON(http.StatusOK, gin.H{
		"nodes":               nodes,
		"total_storage":       totalStorage,
		"used_storage":        usedStorage,
		"replication_factor":  h.cfg.Storage.ReplicationFactor,
		"alive_nodes":         alive,
	})
}

// HealthHandler is a simple liveness check for load balancers.
func (h *Handlers) HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
