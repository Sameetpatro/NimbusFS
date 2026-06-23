package client

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is persisted CLI configuration at ~/.dfs/config.yaml.
type Config struct {
	Server string `yaml:"server"`
	APIKey string `yaml:"api_key"`
	Token  string `yaml:"token"`
}

// DefaultPath returns the default config file location.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dfs", "config.yaml"), nil
}

// Load reads config from path.
func Load(path string) (*Config, error) {
	cfg := &Config{Server: "http://localhost:8080"}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes config to path.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ResolveServer returns server URL with flag > env > config precedence.
func ResolveServer(flagVal string, cfg *Config) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("DFS_SERVER"); v != "" {
		return v
	}
	if cfg.Server != "" {
		return cfg.Server
	}
	return "http://localhost:8080"
}
