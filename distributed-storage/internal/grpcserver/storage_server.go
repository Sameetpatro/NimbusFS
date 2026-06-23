package grpcserver

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
	storagev1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/storagev1"
	"google.golang.org/grpc"
)

const streamFrameSize = 32 * 1024
const storageGRPCTimeout = 30 * time.Second

// StorageGRPCServer implements the proto StorageService interface.
// embedding UnimplementedStorageServiceServer satisfies the interface for forward compatibility
type StorageGRPCServer struct {
	storagev1.UnimplementedStorageServiceServer
	store      *storage.DiskStore
	nodeID     string
	tlsEnabled bool
}

// NewStorageGRPCServer wires disk store into the storage grpc service.
func NewStorageGRPCServer(store *storage.DiskStore, nodeID string, tlsEnabled bool) *StorageGRPCServer {
	return &StorageGRPCServer{store: store, nodeID: nodeID, tlsEnabled: tlsEnabled}
}

// StoreChunk receives a streaming upload and writes to disk.
// streaming lets us handle chunks larger than grpc's 4mb default message limit
func (s *StorageGRPCServer) StoreChunk(stream storagev1.StorageService_StoreChunkServer) error {
	var meta *storagev1.StoreChunkMetadata
	var data []byte

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("grpcserver.StoreChunk: recv: %w", err)
		}

		switch p := req.Payload.(type) {
		case *storagev1.StoreChunkRequest_Metadata:
			meta = p.Metadata
		case *storagev1.StoreChunkRequest_Data:
			data = append(data, p.Data...)
		}
	}

	if meta == nil {
		return fmt.Errorf("grpcserver.StoreChunk: missing metadata")
	}

	chunk := &domain.Chunk{
		ChunkID:  meta.ChunkId,
		FileID:   meta.FileId,
		Index:    int(meta.Index),
		Data:     data,
		Checksum: meta.Checksum,
	}
	if chunk.Checksum == "" {
		chunk.Checksum = meta.ChunkId
	}

	if err := s.store.StoreChunk(chunk); err != nil {
		return fmt.Errorf("grpcserver.StoreChunk: persist %s: %w", meta.ChunkId, err)
	}

	return stream.SendAndClose(&storagev1.StoreChunkResponse{Success: true})
}

// RetrieveChunk reads from disk and streams back.
// chunking the response into 32kb pieces keeps memory flat regardless of chunk size
func (s *StorageGRPCServer) RetrieveChunk(req *storagev1.RetrieveChunkRequest, stream storagev1.StorageService_RetrieveChunkServer) error {
	chunk, err := s.store.RetrieveChunk(req.ChunkId)
	if err != nil {
		return fmt.Errorf("grpcserver.RetrieveChunk: %s: %w", req.ChunkId, err)
	}

	if err := stream.Send(&storagev1.RetrieveChunkResponse{
		Payload: &storagev1.RetrieveChunkResponse_Metadata{
			Metadata: &storagev1.RetrieveChunkMetadata{
				ChunkId:  chunk.ChunkID,
				FileId:   req.FileId,
				Index:    int32(chunk.Index),
				Size:     int64(len(chunk.Data)),
				Checksum: chunk.Checksum,
			},
		},
	}); err != nil {
		return fmt.Errorf("grpcserver.RetrieveChunk: send metadata: %w", err)
	}

	for offset := 0; offset < len(chunk.Data); offset += streamFrameSize {
		end := offset + streamFrameSize
		if end > len(chunk.Data) {
			end = len(chunk.Data)
		}
		if err := stream.Send(&storagev1.RetrieveChunkResponse{
			Payload: &storagev1.RetrieveChunkResponse_Data{Data: chunk.Data[offset:end]},
		}); err != nil {
			return fmt.Errorf("grpcserver.RetrieveChunk: send data: %w", err)
		}
	}
	return nil
}

// DeleteChunk removes a chunk from local disk.
func (s *StorageGRPCServer) DeleteChunk(ctx context.Context, req *storagev1.DeleteChunkRequest) (*storagev1.DeleteChunkResponse, error) {
	if err := s.store.DeleteChunk(req.ChunkId); err != nil {
		return nil, fmt.Errorf("grpcserver.DeleteChunk: %s: %w", req.ChunkId, err)
	}
	return &storagev1.DeleteChunkResponse{Success: true}, nil
}

// Heartbeat acknowledges peer health checks on the storage data plane.
func (s *StorageGRPCServer) Heartbeat(ctx context.Context, req *storagev1.HeartbeatRequest) (*storagev1.HeartbeatResponse, error) {
	_ = ctx
	_ = req
	return &storagev1.HeartbeatResponse{Acknowledged: true}, nil
}

// ReplicateChunk pulls bytes from a healthy replica and stores them locally.
func (s *StorageGRPCServer) ReplicateChunk(ctx context.Context, req *storagev1.ReplicateChunkRequest) (*storagev1.ReplicateChunkResponse, error) {
	if req.SourceAddress == "" {
		return nil, fmt.Errorf("grpcserver.ReplicateChunk: missing source_address")
	}

	callCtx, cancel := context.WithTimeout(ctx, storageGRPCTimeout)
	defer cancel()

	conn, err := grpc.NewClient(req.SourceAddress, DialOptions(s.tlsEnabled)...)
	if err != nil {
		return nil, fmt.Errorf("grpcserver.ReplicateChunk: dial source: %w", err)
	}
	defer conn.Close()

	client := storagev1.NewStorageServiceClient(conn)
	stream, err := client.RetrieveChunk(callCtx, &storagev1.RetrieveChunkRequest{
		ChunkId: req.ChunkId,
		FileId:  req.FileId,
	})
	if err != nil {
		return nil, fmt.Errorf("grpcserver.ReplicateChunk: retrieve: %w", err)
	}

	var meta *storagev1.RetrieveChunkMetadata
	var data []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("grpcserver.ReplicateChunk: recv: %w", err)
		}
		switch p := resp.Payload.(type) {
		case *storagev1.RetrieveChunkResponse_Metadata:
			meta = p.Metadata
		case *storagev1.RetrieveChunkResponse_Data:
			data = append(data, p.Data...)
		}
	}

	if meta == nil {
		return nil, fmt.Errorf("grpcserver.ReplicateChunk: missing metadata")
	}

	chunk := &domain.Chunk{
		ChunkID:  meta.ChunkId,
		FileID:   meta.FileId,
		Index:    int(meta.Index),
		Data:     data,
		Checksum: meta.Checksum,
	}
	if err := s.store.StoreChunk(chunk); err != nil {
		return nil, fmt.Errorf("grpcserver.ReplicateChunk: store: %w", err)
	}

	return &storagev1.ReplicateChunkResponse{Success: true}, nil
}
