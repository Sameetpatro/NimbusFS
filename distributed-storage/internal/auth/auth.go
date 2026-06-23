package auth

import (
	"crypto/tls"
	"crypto/x509"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// Validator will verify JWT and API keys in phase 3 middleware.
type Validator struct {
	secret     []byte
	apiKeyHdr  string
}

// NewValidator builds an auth helper from config; handlers wire this in phase 3.
func NewValidator(cfg config.AuthConfig) *Validator {
	return &Validator{
		secret:    []byte(cfg.JWTSecret),
		apiKeyHdr: cfg.APIKeyHeader,
	}
}

// ParseToken validates a bearer token signature; returns claims map on success.
func (v *Validator) ParseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		// enforce HMAC so callers can't swap in unexpected signing methods
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return v.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

// IssueToken creates a short-lived token for integration tests in later phases.
func (v *Validator) IssueToken(subject string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": subject,
		"exp": time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(v.secret)
}

// TLSCertPool is a placeholder for mutual tls between nodes using golang.org/x/crypto helpers.
func TLSCertPool(pemCerts ...[]byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, pem := range pemCerts {
		if !pool.AppendCertsFromPEM(pem) {
			return nil, x509.ErrUnsupportedAlgorithm
		}
	}
	return pool, nil
}

// MinimumTLSVersion documents the floor we'll enforce when wiring grpc credentials.
var MinimumTLSVersion = tls.VersionTLS12
