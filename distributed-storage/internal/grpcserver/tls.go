package grpcserver

import (
	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ServerOptions returns grpc server options with optional TLS.
func ServerOptions(tlsCfg *tls.Config) []grpc.ServerOption {
	if tlsCfg == nil {
		return nil
	}
	return []grpc.ServerOption{grpc.Creds(credentials.NewTLS(tlsCfg))}
}

// DialOptions returns grpc client dial options with TLS or insecure fallback.
func DialOptions(tlsEnabled bool) []grpc.DialOption {
	if !tlsEnabled {
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}
	return []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		})),
	}
}
