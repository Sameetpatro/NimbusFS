package auth

import "github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"

// ConfigFromSecret builds a minimal AuthConfig for jwt-only validation.
func ConfigFromSecret(secret string) config.AuthConfig {
	return config.AuthConfig{JWTSecret: secret}
}
