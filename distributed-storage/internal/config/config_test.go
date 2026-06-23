package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
)

func TestLoadConfig_FromYAML(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.yaml")
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Master.Port != 8080 {
		t.Errorf("master port: got %d want 8080", cfg.Master.Port)
	}
	if cfg.Storage.ReplicationFactor != 3 {
		t.Errorf("replication factor: got %d want 3", cfg.Storage.ReplicationFactor)
	}
	if cfg.Storage.ChunkSizeMB != 4 {
		t.Errorf("chunk size mb: got %d want 4", cfg.Storage.ChunkSizeMB)
	}
	if cfg.ChunkSizeBytes() != 4*1024*1024 {
		t.Errorf("chunk size bytes: got %d want %d", cfg.ChunkSizeBytes(), 4*1024*1024)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.yaml")

	t.Setenv("NODE_ID", "test-node-42")
	t.Setenv("LOG_LEVEL", "DEBUG")

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Node.NodeID != "test-node-42" {
		t.Errorf("node id: got %q want test-node-42", cfg.Node.NodeID)
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("log level: got %q want debug", cfg.Observability.LogLevel)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := config.LoadConfig(filepath.Join(os.TempDir(), "nonexistent-config.yaml"))
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}
