package unit_test

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

type miniCluster struct {
	router   http.Handler
	store    *metadata.BoltStore
	registry *registry.NodeRegistry
	replMgr  *replication.Manager
	apiKey   string
}

func startMiniCluster(t *testing.T, nodeCount int) *miniCluster {
	t.Helper()

	store, err := metadata.NewBoltStore(t.TempDir())
	if err != nil {
		t.Fatalf("bolt: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	reg := registry.New()
	pool := grpcserver.NewClientPool(logger.New("error"), false)
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
		storagev1.RegisterStorageServiceServer(srv.GRPC(), grpcserver.NewStorageGRPCServer(disk, nodeID, false))
		go func() { _ = srv.ListenAndServe() }()
		t.Cleanup(srv.Stop)

		reg.Register(&domain.StorageNode{
			NodeID: nodeID, Address: addr, Status: domain.NodeStatusAlive,
			LastHeartbeat: time.Now(), TotalSpace: 1 << 30,
		})
	}

	cfg := &config.Config{
		Storage: config.StorageConfig{ReplicationFactor: 3, ChunkSizeMB: 1},
		Auth: config.AuthConfig{
			JWTSecret: "test-secret", APIKeyHeader: "X-API-Key", APIKeys: []string{"test-api-key"},
		},
		API: config.APIConfig{RateLimitRPS: 1000, RateLimitBurst: 1000, MaxUploadMB: 64},
		TLS: config.TLSConfig{Enabled: false},
	}

	replMgr := replication.NewManager(reg, store, pool, cfg.Storage.ReplicationFactor, log)
	metrics := observability.NewMetricsWithRegisterer(prometheus.NewRegistry())
	router := api.NewRouter(cfg, store, reg, replMgr, pool, metrics, log)

	return &miniCluster{router: router, store: store, registry: reg, replMgr: replMgr, apiKey: "test-api-key"}
}

func (c *miniCluster) upload(t *testing.T, data []byte, name string) string {
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

func (c *miniCluster) download(t *testing.T, fileID string) []byte {
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

func TestUploadDownloadDeleteRoundTrip(t *testing.T) {
	cluster := startMiniCluster(t, 3)
	data := bytes.Repeat([]byte("Z"), 8192)
	fileID := cluster.upload(t, data, "unit.bin")
	got := cluster.download(t, fileID)
	if !bytes.Equal(data, got) {
		t.Fatal("roundtrip mismatch")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/"+fileID, nil)
	req.Header.Set("X-API-Key", cluster.apiKey)
	rec := httptest.NewRecorder()
	cluster.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete %d", rec.Code)
	}

	_, err := cluster.store.GetFile(context.Background(), fileID)
	if err == nil {
		t.Fatal("metadata should be gone")
	}
}
