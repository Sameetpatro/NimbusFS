package heartbeat

import (
	"context"
	"fmt"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
	masterv1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/masterv1"
	"google.golang.org/grpc"
)

const (
	grpcCallTimeout = 10 * time.Second
	maxBackoff      = 30 * time.Second
)

// Sender runs on each storage node, calling master's heartbeat rpc on a ticker.
type Sender struct {
	nodeID     string
	masterAddr string
	store      *storage.DiskStore
	interval   time.Duration
	log        *logger.Logger
	tlsEnabled bool
}

// NewSender builds a storage-side heartbeat sender targeting master grpc.
func NewSender(nodeID, masterAddr string, store *storage.DiskStore, interval time.Duration, log *logger.Logger, tlsEnabled bool) *Sender {
	return &Sender{
		nodeID:     nodeID,
		masterAddr: masterAddr,
		store:      store,
		interval:   interval,
		log:        log.WithComponent("heartbeat-sender"),
		tlsEnabled: tlsEnabled,
	}
}

// Start sends heartbeats until context is cancelled.
func (s *Sender) Start(ctx context.Context) error {
	backoff := time.Second

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		client, conn, err := s.dialMaster(ctx)
		if err != nil {
			s.log.Warn("master dial failed", "error", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second

		if err := s.runWithClient(ctx, client); err != nil && ctx.Err() == nil {
			s.log.Warn("heartbeat loop ended", "error", err)
		}
		_ = conn.Close()
	}
}

func (s *Sender) dialMaster(ctx context.Context) (masterv1.MasterServiceClient, *grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
	defer cancel()

	opts := append(grpcserver.DialOptions(s.tlsEnabled), grpc.WithBlock())
	conn, err := grpc.DialContext(dialCtx, s.masterAddr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("heartbeat.Sender.dialMaster: %w", err)
	}
	return masterv1.NewMasterServiceClient(conn), conn, nil
}

func (s *Sender) runWithClient(ctx context.Context, client masterv1.MasterServiceClient) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			used, _ := s.store.UsedBytes()
			total, _ := s.store.TotalBytes()
			count, _ := s.store.ChunkCount()

			callCtx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
			_, err := client.Heartbeat(callCtx, &masterv1.HeartbeatRequest{
				NodeId:     s.nodeID,
				UsedSpace:  used,
				TotalSpace: total,
				ChunkCount: int32(count),
			})
			cancel()

			if err != nil {
				return fmt.Errorf("heartbeat.Sender: send: %w", err)
			}
		}
	}
}

// RegisterWithMaster registers this storage node with exponential backoff until success.
func RegisterWithMaster(ctx context.Context, masterAddr, nodeID, advertiseAddr string, totalSpace int64, log *logger.Logger, tlsEnabled bool) error {
	backoff := time.Second

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		dialCtx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
		opts := append(grpcserver.DialOptions(tlsEnabled), grpc.WithBlock())
		conn, err := grpc.DialContext(dialCtx, masterAddr, opts...)
		cancel()
		if err != nil {
			log.Warn("register dial failed", "error", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		client := masterv1.NewMasterServiceClient(conn)
		callCtx, callCancel := context.WithTimeout(ctx, grpcCallTimeout)
		resp, err := client.RegisterNode(callCtx, &masterv1.RegisterNodeRequest{
			NodeId:     nodeID,
			Address:    advertiseAddr,
			TotalSpace: totalSpace,
		})
		callCancel()
		_ = conn.Close()

		if err != nil {
			log.Warn("register rpc failed", "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		if !resp.Accepted {
			return fmt.Errorf("heartbeat.RegisterWithMaster: rejected: %s", resp.Message)
		}

		log.Info("registered with master", "node_id", resp.AssignedNodeId, "address", advertiseAddr)
		return nil
	}
}
