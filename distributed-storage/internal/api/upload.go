package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/apperrors"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/chunking"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
	"github.com/google/uuid"
)

// UploadService orchestrates the full upload pipeline.
type UploadService struct {
	chunker     *chunking.Chunker
	replMgr     *replication.Manager
	store       metadata.MetadataStore
	grpcPool    *grpcserver.ClientPool
	replFactor  int
	chunkSize   int64
}

// NewUploadService wires upload dependencies.
func NewUploadService(chunker *chunking.Chunker, replMgr *replication.Manager, store metadata.MetadataStore, pool *grpcserver.ClientPool, replFactor int, chunkSize int64) *UploadService {
	return &UploadService{
		chunker:    chunker,
		replMgr:    replMgr,
		store:      store,
		grpcPool:   pool,
		replFactor: replFactor,
		chunkSize:  chunkSize,
	}
}

// UploadResult is returned after a successful upload.
type UploadResult struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	Chunks   int    `json:"chunks"`
}

// Upload runs the full upload pipeline for one file stream.
func (s *UploadService) Upload(ctx context.Context, fileName string, r io.Reader) (*UploadResult, error) {
	// step 1: generate fileID first so we can use it as correlation id in all logs
	fileID := uuid.NewString()

	chunksCh, errCh := s.chunker.Chunk(ctx, fileID, r)

	var chunkInfos []domain.ChunkInfo
	var totalSize int64
	now := time.Now()

	for chunk := range chunksCh {
		// step 3: placement before transfer so we never write bytes without a target
		nodeIDs, err := s.replMgr.StoreChunkReplicas(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("api.UploadService: replicate chunk %d: %w", chunk.Index, err)
		}

		chunkInfos = append(chunkInfos, domain.ChunkInfo{
			ChunkID: chunk.ChunkID,
			Index:   chunk.Index,
			Size:    int64(len(chunk.Data)),
			NodeIDs: nodeIDs,
		})
		totalSize += int64(len(chunk.Data))
	}

	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("api.UploadService: chunking: %w", err)
	}

	meta := &domain.FileMetadata{
		FileID:    fileID,
		FileName:  fileName,
		Size:      totalSize,
		ChunkSize: s.chunkSize,
		Chunks:    chunkInfos,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// step 6: metadata only after all chunks confirmed durable on replicas
	if err := s.store.SaveFile(ctx, meta); err != nil {
		return nil, fmt.Errorf("api.UploadService: saving metadata for %s: %w", meta.FileID, err)
	}

	return &UploadResult{
		FileID:   fileID,
		FileName: fileName,
		Size:     totalSize,
		Chunks:   len(chunkInfos),
	}, nil
}

// IsInsufficientStorage reports whether err is a capacity/placement failure.
func IsInsufficientStorage(err error) bool {
	return errors.Is(err, apperrors.ErrInsufficientStorage)
}
