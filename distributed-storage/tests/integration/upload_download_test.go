//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/api"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/observability"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/replication"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/storage"
	storagev1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/storagev1"
	"github.com/prometheus/client_golang/prometheus"
)

type testCluster struct {
	router   http.Handler
	store    *metadata.BoltStore
	registry *registry.NodeRegistry
	replMgr  *replication.Manager
	apiKey   string
}

func startTestCluster(t *testing.T, nodeCount int) *testCluster {
	t.Helper()

	store, err := metadata.NewBoltStore(t.TempDir())
	if err != nil {
		t.Fatalf("bolt: %v", err)
	}

	reg := registry.New()
	pool := grpcserver.NewClientPool(logger.New("error"))
	log := logger.New("error")

	for i := 0; i < nodeCount; i++ {
		disk, err := storage.NewDiskStore(t.TempDir())
		if err != nil {
			t.Fatalf("disk: %v", err)
		}

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		addr := ln.Addr().String()
		_ = ln.Close()

		nodeID := fmt.Sprintf("node-%d", i)
		srv := grpcserver.NewServer(addr, log)
		storagev1.RegisterStorageServiceServer(srv.GRPC(), grpcserver.NewStorageGRPCServer(disk, nodeID))
		go func() { _ = srv.ListenAndServe() }()
		t.Cleanup(srv.Stop)

		reg.Register(&domain.StorageNode{
			NodeID:        nodeID,
			Address:       addr,
			Status:        domain.NodeStatusAlive,
			LastHeartbeat: time.Now(),
			TotalSpace:    1 << 30,
		})
	}

	cfg := &config.Config{
		Storage: config.StorageConfig{ReplicationFactor: 3, ChunkSizeMB: 1},
		Auth: config.AuthConfig{
			JWTSecret:    "test-secret",
			APIKeyHeader: "X-API-Key",
			APIKeys:      []string{"test-api-key"},
		},
		API: config.APIConfig{RateLimitRPS: 1000, RateLimitBurst: 1000, MaxUploadMB: 64},
	}

	replMgr := replication.NewManager(reg, store, pool, cfg.Storage.ReplicationFactor, log)
	metrics := observability.NewMetricsWithRegisterer(prometheus.NewRegistry())
	router := api.NewRouter(cfg, store, reg, replMgr, pool, metrics, log)

	return &testCluster{router: router, store: store, registry: reg, replMgr: replMgr, apiKey: "test-api-key"}
}

func (c *testCluster) upload(t *testing.T, data []byte, name string) string {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", name)
	_, _ = io.Copy(part, bytes.NewReader(data))
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-API-Key", c.apiKey)

	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status %d body %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp.FileID
}

func (c *testCluster) download(t *testing.T, fileID string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/"+fileID+"/download", nil)
	req.Header.Set("X-API-Key", c.apiKey)
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent && rec.Code != http.StatusOK {
		t.Fatalf("download status %d body %s", rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func TestUploadDownloadRoundTrip(t *testing.T) {
	cluster := startTestCluster(t, 3)
	data := bytes.Repeat([]byte("A"), 10*1024*1024)
	fileID := cluster.upload(t, data, "roundtrip.bin")
	got := cluster.download(t, fileID)
	if !bytes.Equal(data, got) {
		t.Fatalf("roundtrip mismatch: got %d bytes want %d", len(got), len(data))
	}
}

func TestNodeFailureDuringDownload(t *testing.T) {
	cluster := startTestCluster(t, 3)
	data := []byte("failure tolerance test payload")
	fileID := cluster.upload(t, data, "fail.bin")

	meta, err := cluster.store.GetFile(context.Background(), fileID)
	if err != nil {
		t.Fatal(err)
	}
	deadID := meta.Chunks[0].NodeIDs[0]
	cluster.registry.MarkDead(deadID)

	got := cluster.download(t, fileID)
	if !bytes.Equal(data, got) {
		t.Fatal("download failed after node death")
	}
}

func TestReReplicationOnNodeDeath(t *testing.T) {
	cluster := startTestCluster(t, 4)
	data := []byte("re-replication test")
	fileID := cluster.upload(t, data, "repl.bin")

	meta, _ := cluster.store.GetFile(context.Background(), fileID)
	deadID := meta.Chunks[0].NodeIDs[0]
	cluster.registry.MarkDead(deadID)

	cluster.replMgr.ReReplicateFromDeadNode(context.Background(), deadID)

	meta, _ = cluster.store.GetFile(context.Background(), fileID)
	if len(meta.Chunks[0].NodeIDs) != 3 {
		t.Fatalf("expected 3 replicas, got %v", meta.Chunks[0].NodeIDs)
	}
	for _, id := range meta.Chunks[0].NodeIDs {
		if id == deadID {
			t.Fatal("dead node still listed after re-replication")
		}
	}
}

func TestLiveClusterOptional(t *testing.T) {
	if os.Getenv("DFS_SERVER") == "" {
		t.Skip("DFS_SERVER not set — run against docker-compose with DFS_SERVER=http://localhost:8080")
	}
}
