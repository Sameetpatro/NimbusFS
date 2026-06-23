package unit_test

import (
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
	"github.com/prometheus/client_golang/prometheus"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	store, err := metadata.NewBoltStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	reg := registry.New()
	reg.Register(&domain.StorageNode{
		NodeID: "n1", Address: "127.0.0.1:1", Status: domain.NodeStatusAlive,
		LastHeartbeat: time.Now(), TotalSpace: 1000, UsedSpace: 100,
	})

	cfg := &config.Config{
		Storage: config.StorageConfig{ReplicationFactor: 3, ChunkSizeMB: 1},
		Auth: config.AuthConfig{
			JWTSecret: "s", APIKeyHeader: "X-API-Key", APIKeys: []string{"test-key"},
		},
		API: config.APIConfig{RateLimitRPS: 1000, RateLimitBurst: 1000, MaxUploadMB: 8},
		TLS: config.TLSConfig{Enabled: false},
	}
	log := logger.New("error")
	pool := grpcserver.NewClientPool(log, false)
	replMgr := replication.NewManager(reg, store, pool, 3, log)
	metrics := observability.NewMetricsWithRegisterer(prometheus.NewRegistry())
	return api.NewRouter(cfg, store, reg, replMgr, pool, metrics, log)
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	testRouter(t).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}

func TestClusterStatusHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/status", nil)
	w := httptest.NewRecorder()
	testRouter(t).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestListFilesRequiresAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	w := httptest.NewRecorder()
	testRouter(t).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestListFilesWithAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	testRouter(t).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}
