package unit_test

import (
	"testing"
	"time"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/auth"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/logger"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func TestAuthValidator(t *testing.T) {
	v := auth.NewValidator(auth.ConfigFromSecret("test-secret"))
	token, err := v.IssueToken("user-1", 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := v.ParseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != "user-1" {
		t.Fatalf("sub: %v", claims["sub"])
	}
}

func TestLoggerWithFields(t *testing.T) {
	log := logger.New("debug").WithComponent("test").WithNodeID("n1")
	log.Info("msg", "k", "v")
}

func TestMetricsObserve(t *testing.T) {
	m := observability.NewMetricsWithRegisterer(prometheus.NewRegistry())
	m.ObserveUpload(1024)
	m.ObserveDownload(512)
	m.ObserveDelete()
}
