package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// MasterConfig holds master-specific settings passed to the REST and gRPC listeners.
type MasterConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	GRPCPort int    `yaml:"grpc_port"`
	DataDir  string `yaml:"data_dir"`
}

// StorageConfig holds cluster-wide storage policy knobs shared by master and nodes.
type StorageConfig struct {
	ReplicationFactor int    `yaml:"replication_factor"`
	ChunkSizeMB       int    `yaml:"chunk_size_mb"`
	DataDir           string `yaml:"data_dir"`
}

// NodeConfig holds per-node runtime settings for heartbeats and identity.
type NodeConfig struct {
	HeartbeatInterval int    `yaml:"heartbeat_interval"`
	DeadThreshold     int    `yaml:"dead_threshold"`
	NodeID            string `yaml:"node_id"`
}

// AuthConfig holds REST authentication settings for later middleware wiring.
type AuthConfig struct {
	JWTSecret    string `yaml:"jwt_secret"`
	APIKeyHeader string `yaml:"api_key_header"`
}

// ObservabilityConfig holds logging and metrics ports for prometheus scraping.
type ObservabilityConfig struct {
	MetricsPort int    `yaml:"metrics_port"`
	LogLevel    string `yaml:"log_level"`
}

// Config is the top-level config struct, mirroring config.yaml shape exactly.
// using nested structs instead of flat keys lets us pass sub-configs to subsystems
type Config struct {
	Master        MasterConfig
	Storage       StorageConfig
	Node          NodeConfig
	Auth          AuthConfig
	Observability ObservabilityConfig
}

// LoadConfig reads yaml from path then overlays env vars.
// env vars win over file values so docker/k8s deployments can override without rebuilding images
func LoadConfig(path string) (*Config, error) {
	// start from empty struct so missing yaml keys don't leave stale defaults from a prior load
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		// wrap with path so operators know which file failed in multi-env setups
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		// yaml errors are often line-number opaque; %w preserves the underlying parse detail
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	// overlay env after file so container orchestrators can inject secrets and ids at runtime
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides maps well-known env vars onto cfg fields.
// we use a dedicated function so LoadConfig stays readable as more vars get added in later phases
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("MASTER_HOST"); v != "" {
		cfg.Master.Host = v
	}
	if v := os.Getenv("MASTER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Master.Port = n
		}
	}
	if v := os.Getenv("MASTER_GRPC_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Master.GRPCPort = n
		}
	}
	if v := os.Getenv("MASTER_DATA_DIR"); v != "" {
		cfg.Master.DataDir = v
	}

	if v := os.Getenv("STORAGE_REPLICATION_FACTOR"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Storage.ReplicationFactor = n
		}
	}
	if v := os.Getenv("STORAGE_CHUNK_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Storage.ChunkSizeMB = n
		}
	}
	if v := os.Getenv("STORAGE_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}

	if v := os.Getenv("NODE_HEARTBEAT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Node.HeartbeatInterval = n
		}
	}
	if v := os.Getenv("NODE_DEAD_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Node.DeadThreshold = n
		}
	}
	// NODE_ID is the primary identity override for storage containers in docker-compose
	if v := os.Getenv("NODE_ID"); v != "" {
		cfg.Node.NodeID = v
	}

	if v := os.Getenv("AUTH_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("AUTH_API_KEY_HEADER"); v != "" {
		cfg.Auth.APIKeyHeader = v
	}

	if v := os.Getenv("METRICS_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Observability.MetricsPort = n
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		// normalize to lowercase so "INFO" from k8s config maps still works with slog parsers
		cfg.Observability.LogLevel = strings.ToLower(v)
	}
}

// ChunkSizeBytes converts the mb config knob to bytes for chunker and domain logic.
func (c *Config) ChunkSizeBytes() int64 {
	// multiply by 1024^2 not 1e6 so we align with how os and disks report block sizes
	return int64(c.Storage.ChunkSizeMB) * 1024 * 1024
}

// MasterRESTAddr returns host:port for gin to bind, keeping format logic in one place.
func (c *Config) MasterRESTAddr() string {
	return fmt.Sprintf("%s:%d", c.Master.Host, c.Master.Port)
}

// MasterGRPCAddr returns host:port for the master gRPC server listener.
func (c *Config) MasterGRPCAddr() string {
	return fmt.Sprintf("%s:%d", c.Master.Host, c.Master.GRPCPort)
}
