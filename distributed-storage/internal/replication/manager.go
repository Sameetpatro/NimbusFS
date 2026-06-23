package replication

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/apperrors"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/chunking"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	storagev1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/storagev1"
	"golang.org/x/sync/errgroup"
)

const grpcTimeout = 30 * time.Second

// ErrInsufficientStorage is re-exported for callers that already import replication.
var ErrInsufficientStorage = apperrors.ErrInsufficientStorage

// Manager handles all replication decisions.
// keeping this separate from master api logic means replication can be tested independently
type Manager struct {
	registry     *registry.NodeRegistry
	store        metadata.MetadataStore
	grpcPool     *grpcserver.ClientPool
	log          *logger.Logger
	replFactor   int
	pipelineSize int
	replLag      int64 // atomic-ish counter for chunks awaiting re-replication
}

// ReplicationFactor returns configured replica count for uploads.
func (m *Manager) ReplicationFactor() int {
	return m.replFactor
}

// ReplicationLag returns chunks currently waiting for re-replication.
func (m *Manager) ReplicationLag() float64 {
	return float64(m.replLag)
}

// StoreChunkReplicas places one chunk on n healthy nodes concurrently.
func (m *Manager) StoreChunkReplicas(ctx context.Context, chunk *domain.Chunk) ([]string, error) {
	nodes, err := m.SelectNodesForChunk(m.replFactor)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperrors.ErrInsufficientStorage, err)
	}

	g, gctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, m.pipelineSize)
	nodeIDs := make([]string, len(nodes))

	for i, node := range nodes {
		i, node := i, node
		nodeIDs[i] = node.NodeID
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			return m.pushDomainChunk(gctx, node, chunk)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("replication.StoreChunkReplicas: %w", err)
	}
	return nodeIDs, nil
}

// FetchChunkWithFallback tries replicas in order until one succeeds.
func (m *Manager) FetchChunkWithFallback(ctx context.Context, fileID string, info domain.ChunkInfo) ([]byte, error) {
	var lastErr error
	for _, nodeID := range info.NodeIDs {
		node, ok := m.registry.Get(nodeID)
		if !ok || node.Status == domain.NodeStatusDead {
			continue
		}
		data, err := m.fetchChunk(ctx, node, fileID, info.ChunkID)
		if err != nil {
			lastErr = err
			continue
		}
		if err := chunking.VerifyBytesChecksum(data, info.ChunkID); err != nil {
			lastErr = err
			continue
		}
		return data, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrChunkUnavailable, lastErr)
	}
	return nil, apperrors.ErrChunkUnavailable
}

// DeleteChunkReplicas sends delete rpc to every node holding the chunk.
func (m *Manager) DeleteChunkReplicas(ctx context.Context, fileID, chunkID string, nodeIDs []string) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, nodeID := range nodeIDs {
		nodeID := nodeID
		g.Go(func() error {
			node, ok := m.registry.Get(nodeID)
			if !ok {
				return nil
			}
			callCtx, cancel := context.WithTimeout(gctx, grpcTimeout)
			defer cancel()

			client, err := m.grpcPool.GetClient(node.NodeID, node.Address)
			if err != nil {
				return err
			}
			_, err = client.DeleteChunk(callCtx, &storagev1.DeleteChunkRequest{
				ChunkId: chunkID,
				FileId:  fileID,
			})
			return err
		})
	}
	return g.Wait()
}

func (m *Manager) pushDomainChunk(ctx context.Context, target *domain.StorageNode, chunk *domain.Chunk) error {
	info := domain.ChunkInfo{
		ChunkID: chunk.ChunkID,
		Index:   chunk.Index,
		Size:    int64(len(chunk.Data)),
	}
	return m.pushChunk(ctx, target, chunk.FileID, info, chunk.Data)
}

// NewManager wires replication dependencies from master startup.
func NewManager(reg *registry.NodeRegistry, store metadata.MetadataStore, pool *grpcserver.ClientPool, replFactor int, log *logger.Logger) *Manager {
	return &Manager{
		registry:     reg,
		store:        store,
		grpcPool:     pool,
		log:          log.WithComponent("replication"),
		replFactor:   replFactor,
		pipelineSize: replFactor,
	}
}

// ListenDeadNodes consumes dead node events and triggers re-replication in background goroutines.
func (m *Manager) ListenDeadNodes(ctx context.Context, deadCh <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case deadID, ok := <-deadCh:
			if !ok {
				return
			}
			// runs in its own goroutine because it may involve many grpc calls
			go m.ReReplicateFromDeadNode(ctx, deadID)
		}
	}
}

// SelectNodesForChunk picks n healthy nodes for a new chunk placement.
// algorithm: sort by free space descending, pick top n, shuffle to avoid hotspots
func (m *Manager) SelectNodesForChunk(n int) ([]*domain.StorageNode, error) {
	candidates := m.registry.ListAlive()
	if len(candidates) < n {
		return nil, fmt.Errorf("replication.SelectNodesForChunk: need %d nodes, have %d alive", n, len(candidates))
	}

	sort.Slice(candidates, func(i, j int) bool {
		freeI := candidates[i].TotalSpace - candidates[i].UsedSpace
		freeJ := candidates[j].TotalSpace - candidates[j].UsedSpace
		return freeI > freeJ
	})

	top := candidates[:n]
	// shuffle top n so we don't always hammer the same highest-capacity node first
	rand.Shuffle(len(top), func(i, j int) { top[i], top[j] = top[j], top[i] })
	return top, nil
}

// ReReplicateFromDeadNode finds all chunks on deadNodeID and replicates them elsewhere.
func (m *Manager) ReReplicateFromDeadNode(ctx context.Context, deadNodeID string) {
	m.log.Info("starting re-replication", "dead_node", deadNodeID)
	m.replLag++

	files, err := m.store.ListFiles(ctx)
	if err != nil {
		m.log.Error("re-replication list files failed", "error", err)
		return
	}

	// sync.WaitGroup waits for all chunk copies to finish before we log completion
	var wg sync.WaitGroup
	for _, file := range files {
		for _, chunk := range file.Chunks {
			if !containsNode(chunk.NodeIDs, deadNodeID) {
				continue
			}
			wg.Add(1)
			go func(fileID string, info domain.ChunkInfo) {
				defer wg.Done()
				if err := m.replicateChunk(ctx, fileID, info, deadNodeID); err != nil {
					m.log.Error("chunk re-replication failed", "file_id", fileID, "chunk_id", info.ChunkID, "error", err)
				}
			}(file.FileID, chunk)
		}
	}
	wg.Wait()
	m.replLag--
	m.log.Info("re-replication complete", "dead_node", deadNodeID)
}

func (m *Manager) replicateChunk(ctx context.Context, fileID string, chunk domain.ChunkInfo, deadNodeID string) error {
	source, err := m.pickSourceReplica(chunk.NodeIDs, deadNodeID)
	if err != nil {
		return fmt.Errorf("replication.replicateChunk: %w", err)
	}

	newNodes, err := m.selectReplacementNodes(chunk.NodeIDs, deadNodeID, 1)
	if err != nil {
		return fmt.Errorf("replication.replicateChunk: select targets: %w", err)
	}

	data, err := m.fetchChunk(ctx, source, fileID, chunk.ChunkID)
	if err != nil {
		return fmt.Errorf("replication.replicateChunk: fetch: %w", err)
	}

	// errgroup.Group for concurrent chunk uploads — cancel all if one fails
	g, gctx := errgroup.WithContext(ctx)
	// buffered channel (size = replication_factor) limits in-flight grpc store calls
	sem := make(chan struct{}, m.pipelineSize)
	for _, target := range newNodes {
		t := target
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			return m.pushChunk(gctx, t, fileID, chunk, data)
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("replication.replicateChunk: push: %w", err)
	}

	updated := replaceDeadNode(chunk.NodeIDs, deadNodeID, newNodes[0].NodeID)
	if err := m.store.UpdateChunkNodes(ctx, fileID, chunk.ChunkID, updated); err != nil {
		return fmt.Errorf("replication.replicateChunk: update metadata: %w", err)
	}
	return nil
}

func (m *Manager) pickSourceReplica(nodeIDs []string, deadNodeID string) (*domain.StorageNode, error) {
	for _, id := range nodeIDs {
		if id == deadNodeID {
			continue
		}
		node, ok := m.registry.Get(id)
		if ok && node.Status == domain.NodeStatusAlive {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no alive replica for chunk")
}

func (m *Manager) selectReplacementNodes(existing []string, deadNodeID string, need int) ([]*domain.StorageNode, error) {
	exclude := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		if id != deadNodeID {
			exclude[id] = struct{}{}
		}
	}

	var candidates []*domain.StorageNode
	for _, node := range m.registry.ListAlive() {
		if _, skip := exclude[node.NodeID]; skip {
			continue
		}
		candidates = append(candidates, node)
	}

	if len(candidates) < need {
		return nil, fmt.Errorf("not enough replacement nodes")
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].TotalSpace-candidates[i].UsedSpace > candidates[j].TotalSpace-candidates[j].UsedSpace
	})
	return candidates[:need], nil
}

func (m *Manager) fetchChunk(ctx context.Context, source *domain.StorageNode, fileID, chunkID string) ([]byte, error) {
	callCtx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	client, err := m.grpcPool.GetClient(source.NodeID, source.Address)
	if err != nil {
		return nil, err
	}

	stream, err := client.RetrieveChunk(callCtx, &storagev1.RetrieveChunkRequest{
		ChunkId: chunkID,
		FileId:  fileID,
	})
	if err != nil {
		return nil, err
	}

	var data []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("replication.fetchChunk: recv: %w", err)
		}
		switch p := resp.Payload.(type) {
		case *storagev1.RetrieveChunkResponse_Data:
			data = append(data, p.Data...)
		}
	}
	return data, nil
}

func (m *Manager) pushChunk(ctx context.Context, target *domain.StorageNode, fileID string, chunk domain.ChunkInfo, data []byte) error {
	callCtx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()

	client, err := m.grpcPool.GetClient(target.NodeID, target.Address)
	if err != nil {
		return err
	}

	stream, err := client.StoreChunk(callCtx)
	if err != nil {
		return err
	}

	if err := stream.Send(&storagev1.StoreChunkRequest{
		Payload: &storagev1.StoreChunkRequest_Metadata{
			Metadata: &storagev1.StoreChunkMetadata{
				ChunkId:  chunk.ChunkID,
				FileId:   fileID,
				Index:    int32(chunk.Index),
				Size:     chunk.Size,
				Checksum: chunk.ChunkID,
			},
		},
	}); err != nil {
		return err
	}

	const frameSize = 32 * 1024
	for offset := 0; offset < len(data); offset += frameSize {
		end := offset + frameSize
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&storagev1.StoreChunkRequest{
			Payload: &storagev1.StoreChunkRequest_Data{Data: data[offset:end]},
		}); err != nil {
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	return err
}

func containsNode(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func replaceDeadNode(ids []string, deadID, replacement string) []string {
	out := make([]string, 0, len(ids))
	replaced := false
	for _, id := range ids {
		if id == deadID {
			if !replaced {
				out = append(out, replacement)
				replaced = true
			}
			continue
		}
		out = append(out, id)
	}
	if !replaced {
		out = append(out, replacement)
	}
	return out
}
