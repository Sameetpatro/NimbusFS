package api

import (
	"context"
	"fmt"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/apperrors"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
)

// DeleteService removes file metadata and chunk replicas.
type DeleteService struct {
	store   metadata.MetadataStore
	replMgr *replication.Manager
}

// NewDeleteService wires delete dependencies.
func NewDeleteService(store metadata.MetadataStore, replMgr *replication.Manager) *DeleteService {
	return &DeleteService{store: store, replMgr: replMgr}
}

// Delete removes metadata and sends delete rpc to all nodes holding chunks.
func (s *DeleteService) Delete(ctx context.Context, fileID string) error {
	meta, err := s.store.GetFile(ctx, fileID)
	if err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrFileNotFound, err)
	}

	for _, chunk := range meta.Chunks {
		if err := s.replMgr.DeleteChunkReplicas(ctx, fileID, chunk.ChunkID, chunk.NodeIDs); err != nil {
			return fmt.Errorf("api.DeleteService: delete chunk %s: %w", chunk.ChunkID, err)
		}
	}

	if err := s.store.DeleteFile(ctx, fileID); err != nil {
		return fmt.Errorf("api.DeleteService: delete metadata %s: %w", fileID, err)
	}
	return nil
}
