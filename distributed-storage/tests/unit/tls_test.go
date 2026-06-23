package unit_test

import (
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/tlsconfig"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	cert, err := tlsconfig.GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 || cert.PrivateKey == nil {
		t.Fatal("expected cert material")
	}
}

func TestLoadOrGenerateTLS(t *testing.T) {
	dir := t.TempDir()
	certFile := dir + "/server.pem"
	keyFile := dir + "/server.key"

	cfg, err := tlsconfig.LoadOrGenerateTLS(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) == 0 {
		t.Fatal("expected server certificates")
	}

	cfg2, err := tlsconfig.LoadOrGenerateTLS(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2 == nil {
		t.Fatal("expected reload")
	}
}
