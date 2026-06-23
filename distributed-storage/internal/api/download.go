package api

import (
	"context"
	"fmt"
	"io"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/apperrors"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
)

// DownloadService fetches chunks in order and streams to writer.
type DownloadService struct {
	store    metadata.MetadataStore
	grpcPool *grpcserver.ClientPool
	replMgr  *replication.Manager
}

// NewDownloadService wires download dependencies.
func NewDownloadService(store metadata.MetadataStore, pool *grpcserver.ClientPool, replMgr *replication.Manager) *DownloadService {
	return &DownloadService{store: store, grpcPool: pool, replMgr: replMgr}
}

// Stream writes file bytes to w in chunk order.
func (s *DownloadService) Stream(ctx context.Context, fileID string, w io.Writer) (*domain.FileMetadata, error) {
	meta, err := s.store.GetFile(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrFileNotFound, err)
	}

	for _, info := range meta.Chunks {
		data, err := s.replMgr.FetchChunkWithFallback(ctx, fileID, info)
		if err != nil {
			return nil, fmt.Errorf("api.DownloadService: chunk %s: %w", info.ChunkID, err)
		}

		// write immediately — memory stays O(chunk_size) not O(file_size)
		if _, err := w.Write(data); err != nil {
			return nil, fmt.Errorf("api.DownloadService: write chunk %d: %w", info.Index, err)
		}
	}

	return meta, nil
}
