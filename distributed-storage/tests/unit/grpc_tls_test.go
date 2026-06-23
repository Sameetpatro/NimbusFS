package unit_test

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/tlsconfig"
)

func TestGRPCServerOptions(t *testing.T) {
	if len(grpcserver.ServerOptions(nil)) != 0 {
		t.Fatal("nil tls should return no options")
	}
	cert, err := tlsconfig.GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	if len(grpcserver.ServerOptions(cfg)) != 1 {
		t.Fatal("expected creds option")
	}
}

func TestGRPCDialOptions(t *testing.T) {
	if len(grpcserver.DialOptions(false)) != 1 {
		t.Fatal("insecure dial")
	}
	if len(grpcserver.DialOptions(true)) != 1 {
		t.Fatal("tls dial")
	}
}

func TestClientTLSConfig(t *testing.T) {
	cfg := tlsconfig.ClientTLSConfig()
	if cfg.MinVersion != tls.VersionTLS12 || !cfg.InsecureSkipVerify {
		t.Fatalf("unexpected client tls: %#v", cfg)
	}
}

func TestMetadataListFiles(t *testing.T) {
	store, err := metadata.NewBoltStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	files, err := store.ListFiles(ctx)
	if err != nil || len(files) != 0 {
		t.Fatalf("empty list: %v %d", err, len(files))
	}
}
