package unit_test

import (
	"context"
	"net"
	"testing"

	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/domain"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/grpcserver"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/metadata"
	"github.com/Sameetpatro/NimbusFS/distributed-storage/internal/registry"
	masterv1 "github.com/Sameetpatro/NimbusFS/distributed-storage/proto/gen/masterv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMasterGRPCRegisterAndHeartbeat(t *testing.T) {
	store, err := metadata.NewBoltStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	reg := registry.New()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	masterv1.RegisterMasterServiceServer(srv, grpcserver.NewMasterGRPCServer(reg, store, store))
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := masterv1.NewMasterServiceClient(conn)

	ctx := context.Background()
	resp, err := client.RegisterNode(ctx, &masterv1.RegisterNodeRequest{
		NodeId: "n1", Address: "127.0.0.1:1", TotalSpace: 1000,
	})
	if err != nil || !resp.Accepted {
		t.Fatalf("register: %v %#v", err, resp)
	}

	hb, err := client.Heartbeat(ctx, &masterv1.HeartbeatRequest{
		NodeId: "n1", UsedSpace: 10, TotalSpace: 1000, ChunkCount: 1,
	})
	if err != nil || !hb.Acknowledged {
		t.Fatalf("heartbeat: %v %#v", err, hb)
	}

	nodes, _ := store.ListNodes(ctx)
	if len(nodes) != 1 {
		t.Fatalf("persisted nodes %d", len(nodes))
	}

	node, ok := reg.Get("n1")
	if !ok || node.Status != domain.NodeStatusAlive {
		t.Fatalf("registry: %#v", node)
	}
}
