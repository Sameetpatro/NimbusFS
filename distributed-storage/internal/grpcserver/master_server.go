package grpcserver

import (
	"context"
	"fmt"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	masterv1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/masterv1"
)

// MasterGRPCServer handles node registrations and heartbeats from storage nodes.
type MasterGRPCServer struct {
	masterv1.UnimplementedMasterServiceServer
	registry *registry.NodeRegistry
	store    metadata.MetadataStore
	bolt     *metadata.BoltStore
}

// NewMasterGRPCServer wires registry and persistence into master grpc handlers.
func NewMasterGRPCServer(reg *registry.NodeRegistry, store metadata.MetadataStore, bolt *metadata.BoltStore) *MasterGRPCServer {
	return &MasterGRPCServer{registry: reg, store: store, bolt: bolt}
}

// RegisterNode is called by storage nodes on startup.
// idempotent — if a node re-registers after crash, update its record not create duplicate
func (s *MasterGRPCServer) RegisterNode(ctx context.Context, req *masterv1.RegisterNodeRequest) (*masterv1.RegisterNodeResponse, error) {
	nodeID := req.NodeId
	if nodeID == "" {
		return &masterv1.RegisterNodeResponse{Accepted: false, Message: "node_id required"}, nil
	}

	now := time.Now()
	node := &domain.StorageNode{
		NodeID:        nodeID,
		Address:       req.Address,
		Status:        domain.NodeStatusAlive,
		LastHeartbeat: now,
		TotalSpace:    req.TotalSpace,
	}

	s.registry.Register(node)

	if s.bolt != nil {
		if err := s.bolt.SaveNode(ctx, node); err != nil {
			return nil, fmt.Errorf("grpcserver.RegisterNode: persist %s: %w", nodeID, err)
		}
	}

	return &masterv1.RegisterNodeResponse{
		AssignedNodeId: nodeID,
		Accepted:       true,
		Message:        "registered",
	}, nil
}

// Heartbeat updates LastHeartbeat and disk stats for a node.
func (s *MasterGRPCServer) Heartbeat(ctx context.Context, req *masterv1.HeartbeatRequest) (*masterv1.HeartbeatResponse, error) {
	if !s.registry.UpdateHeartbeat(req.NodeId, req.UsedSpace, req.TotalSpace, int(req.ChunkCount)) {
		return &masterv1.HeartbeatResponse{Acknowledged: false}, nil
	}

	// persist updated capacity stats so master crash recovery has fresh placement data
	if s.bolt != nil {
		if node, ok := s.registry.Get(req.NodeId); ok {
			if err := s.bolt.SaveNode(ctx, node); err != nil {
				return nil, fmt.Errorf("grpcserver.Heartbeat: persist %s: %w", req.NodeId, err)
			}
		}
	}

	return &masterv1.HeartbeatResponse{Acknowledged: true}, nil
}

// ReportChunkStored acknowledges chunk placement reports for phase 3 upload path.
func (s *MasterGRPCServer) ReportChunkStored(ctx context.Context, req *masterv1.ReportChunkStoredRequest) (*masterv1.ReportChunkStoredResponse, error) {
	_ = ctx
	_ = req
	return &masterv1.ReportChunkStoredResponse{Acknowledged: true}, nil
}
