// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

func TestGRPCRemoteClient(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	registerTestGRPCServices(t, s)
	go s.Serve(lis)
	defer s.Stop()

	client, err := NewGRPCRemoteClient(&GRPCRemoteClientConfig{
		Addr:    lis.Addr().String(),
		Plugins: testGRPCPluginMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}

	raw, err := client.Dispense("test")
	if err != nil {
		t.Fatal(err)
	}

	impl, ok := raw.(testInterface)
	if !ok {
		t.Fatalf("expected testInterface, got %T", raw)
	}

	result := impl.Double(21)
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
}

func TestGRPCRemoteClient_WithKeepalive(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(DefaultGRPCServerKeepaliveEnforcementPolicy()),
	)
	registerTestGRPCServices(t, s)
	go s.Serve(lis)
	defer s.Stop()

	client, err := NewGRPCRemoteClient(&GRPCRemoteClientConfig{
		Addr:    lis.Addr().String(),
		Plugins: testGRPCPluginMap,
		KeepaliveParams: &keepalive.ClientParameters{
			Time:                1 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCRemoteClient_HealthCheck(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	healthCheck := health.NewServer()
	healthCheck.SetServingStatus(GRPCServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthCheck)
	go s.Serve(lis)
	defer s.Stop()

	err = GRPCRemoteClientHealthCheck(context.Background(), lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
}

func TestGRPCRemoteClient_HealthCheckFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := GRPCRemoteClientHealthCheck(ctx, "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected health check to fail for non-existent server")
	}
}

func TestGRPCRemoteClient_ServerNotRunning(t *testing.T) {
	client, err := NewGRPCRemoteClient(&GRPCRemoteClientConfig{
		Addr:    "127.0.0.1:1",
		Plugins: testGRPCPluginMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Ping(); err == nil {
		t.Fatal("expected ping to fail when server is not running")
	}
}

func registerTestGRPCServices(t *testing.T, s *grpc.Server) {
	t.Helper()

	healthCheck := health.NewServer()
	healthCheck.SetServingStatus(GRPCServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthCheck)

	testPlugin := &testGRPCInterfacePlugin{}
	if err := testPlugin.GRPCServer(nil, s); err != nil {
		t.Fatal(err)
	}
}
