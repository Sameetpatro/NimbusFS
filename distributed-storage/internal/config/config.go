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
	GRPCAddr string `yaml:"-"`
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
	AdvertiseAddr     string `yaml:"-"`
}

// AuthConfig holds REST authentication settings.
type AuthConfig struct {
	JWTSecret    string   `yaml:"jwt_secret"`
	APIKeyHeader string   `yaml:"api_key_header"`
	APIKeys      []string `yaml:"api_keys"`
}

// APIConfig holds REST API tuning knobs.
type APIConfig struct {
	RateLimitRPS   int `yaml:"rate_limit_rps"`
	RateLimitBurst int `yaml:"rate_limit_burst"`
	MaxUploadMB    int `yaml:"max_upload_mb"`
}

// ObservabilityConfig holds logging and metrics ports for prometheus scraping.
type ObservabilityConfig struct {
	MetricsPort int    `yaml:"metrics_port"`
	LogLevel    string `yaml:"log_level"`
}

// TLSConfig controls optional mTLS between nodes.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// Config is the top-level config struct, mirroring config.yaml shape exactly.
type Config struct {
	Master        MasterConfig
	Storage       StorageConfig
	Node          NodeConfig
	Auth          AuthConfig
	API           APIConfig
	TLS           TLSConfig
	Observability ObservabilityConfig
}

// LoadConfig reads yaml from path then overlays env vars.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		API: APIConfig{
			RateLimitRPS:   100,
			RateLimitBurst: 200,
			MaxUploadMB:    32,
		},
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "/data/certs/server.pem",
			KeyFile:  "/data/certs/server.key",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	applyEnvOverrides(cfg)
	applyAPIDefaults(cfg)
	return cfg, nil
}

func applyAPIDefaults(cfg *Config) {
	if cfg.API.RateLimitRPS == 0 {
		cfg.API.RateLimitRPS = 100
	}
	if cfg.API.RateLimitBurst == 0 {
		cfg.API.RateLimitBurst = 200
	}
	if cfg.API.MaxUploadMB == 0 {
		cfg.API.MaxUploadMB = 32
	}
	if cfg.Auth.APIKeyHeader == "" {
		cfg.Auth.APIKeyHeader = "X-API-Key"
	}
}

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
	if v := os.Getenv("NODE_ID"); v != "" {
		cfg.Node.NodeID = v
	}
	if v := os.Getenv("AUTH_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("AUTH_API_KEY_HEADER"); v != "" {
		cfg.Auth.APIKeyHeader = v
	}
	if v := os.Getenv("AUTH_API_KEYS"); v != "" {
		cfg.Auth.APIKeys = strings.Split(v, ",")
	}
	if v := os.Getenv("METRICS_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Observability.MetricsPort = n
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Observability.LogLevel = strings.ToLower(v)
	}
	if v := os.Getenv("MASTER_GRPC_ADDR"); v != "" {
		cfg.Master.GRPCAddr = v
	}
	if v := os.Getenv("STORAGE_GRPC_ADDR"); v != "" {
		cfg.Node.AdvertiseAddr = v
	}
	if v := os.Getenv("TLS_ENABLED"); v != "" {
		cfg.TLS.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("TLS_CERT_FILE"); v != "" {
		cfg.TLS.CertFile = v
	}
	if v := os.Getenv("TLS_KEY_FILE"); v != "" {
		cfg.TLS.KeyFile = v
	}
}

func (c *Config) ChunkSizeBytes() int64 {
	return int64(c.Storage.ChunkSizeMB) * 1024 * 1024
}

func (c *Config) MaxUploadBytes() int64 {
	return int64(c.API.MaxUploadMB) * 1024 * 1024
}

func (c *Config) MasterRESTAddr() string {
	return fmt.Sprintf("%s:%d", c.Master.Host, c.Master.Port)
}

func (c *Config) MasterGRPCListenAddr() string {
	return fmt.Sprintf("%s:%d", c.Master.Host, c.Master.GRPCPort)
}

func (c *Config) MasterGRPCAddr() string {
	if c.Master.GRPCAddr != "" {
		return c.Master.GRPCAddr
	}
	return fmt.Sprintf("%s:%d", c.Master.Host, c.Master.GRPCPort)
}

func (c *Config) StorageAdvertiseAddr(grpcPort int) string {
	if c.Node.AdvertiseAddr != "" {
		return c.Node.AdvertiseAddr
	}
	return fmt.Sprintf("0.0.0.0:%d", grpcPort)
}
