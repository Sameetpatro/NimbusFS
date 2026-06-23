package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// GenerateSelfSignedCert creates a cert+key pair for dev/test deployments.
// using ecdsa p256 over rsa because shorter keys, faster handshakes, same security level
func GenerateSelfSignedCert(host string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("tlsconfig.GenerateSelfSignedCert: key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("tlsconfig.GenerateSelfSignedCert: serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"NimbusFS Dev"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else if host != "" {
		template.DNSNames = []string{host, "localhost"}
	} else {
		template.DNSNames = []string{"localhost"}
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("tlsconfig.GenerateSelfSignedCert: create: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}, nil
}

// LoadOrGenerateTLS returns a tls.Config, loading from disk if files exist else generating.
func LoadOrGenerateTLS(certFile, keyFile string) (*tls.Config, error) {
	if err := os.MkdirAll(filepath.Dir(certFile), 0o755); err != nil {
		return nil, fmt.Errorf("tlsconfig.LoadOrGenerateTLS: mkdir: %w", err)
	}

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		cert, err := GenerateSelfSignedCert("localhost")
		if err != nil {
			return nil, err
		}
		if err := writeCertFiles(certFile, keyFile, cert); err != nil {
			return nil, err
		}
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tlsconfig.LoadOrGenerateTLS: load pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		// self-signed dev certs: skip verify on client side in grpc dial helpers
		InsecureSkipVerify: false,
	}, nil
}

// ClientTLSConfig returns tls config for grpc clients connecting to self-signed servers.
func ClientTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	}
}

func writeCertFiles(certFile, keyFile string, cert tls.Certificate) error {
	if len(cert.Certificate) == 0 {
		return fmt.Errorf("tlsconfig.writeCertFiles: empty certificate")
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		return err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	return os.WriteFile(keyFile, keyPEM, 0o600)
}
