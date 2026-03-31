// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

// This is an example of a remote plugin server that serves a gRPC plugin
// over the network. Unlike local plugins which are subprocess-based,
// this server can be accessed from anywhere on the network.
//
// Usage:
//
//	go run . -addr :50051
//
// This server uses keepalive to maintain reliable connections over the
// network, making it suitable for cross-machine plugin communication.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-plugin/examples/remote/shared"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	addr := flag.String("addr", ":50051", "Address to listen on (e.g. :50051 or 0.0.0.0:50051)")
	flag.Parse()

	// Create a TCP listener for the network
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	fmt.Printf("Remote plugin server listening on %s\n", lis.Addr().String())

	// Create a gRPC server with keepalive enforcement for reliable
	// network connections. This ensures the server accepts keepalive
	// pings from remote clients.
	grpcServer := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(
			plugin.DefaultGRPCServerKeepaliveEnforcementPolicy(),
		),
	)

	// Register health check service - this allows clients to verify
	// the server is available before attempting to use plugins.
	healthCheck := health.NewServer()
	healthCheck.SetServingStatus(
		plugin.GRPCServiceName,
		grpc_health_v1.HealthCheckResponse_SERVING,
	)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthCheck)

	// Register our plugin
	kvPlugin := &shared.KVGRPCPlugin{Impl: shared.NewInMemoryKV()}
	if err := kvPlugin.GRPCServer(nil, grpcServer); err != nil {
		log.Fatalf("Failed to register plugin: %v", err)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down server...")
		grpcServer.GracefulStop()
	}()

	fmt.Println("Server ready. Clients can connect to:", lis.Addr().String())
	fmt.Println("Press Ctrl+C to stop.")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
