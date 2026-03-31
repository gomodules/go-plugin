// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

// This is an example of a remote plugin client that connects to a gRPC
// plugin server over the network. Unlike local plugins which are subprocess-based,
// this client connects to a remote server that may be on a different machine.
//
// The client uses keepalive to maintain reliable connections over potentially
// unreliable network connections.
//
// Usage:
//
//	go run . -addr localhost:50051 [get|put] [args...]
//
// Example:
//
//	# Start the server in one terminal:
//	cd server && go run . -addr :50051
//
//	# In another terminal, put a value:
//	cd client && go run . -addr localhost:50051 put mykey "hello world"
//
//	# Get the value:
//	cd client && go run . -addr localhost:50051 get mykey
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-plugin/examples/remote/shared"
)

func main() {
	addr := flag.String("addr", "localhost:50051", "Address of the remote plugin server")
	healthCheckOnly := flag.Bool("health", false, "Only perform health check and exit")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 && !*healthCheckOnly {
		fmt.Println("Usage: client -addr <address> [get|put] [args...]")
		fmt.Println("       client -addr <address> -health")
		os.Exit(1)
	}

	// Optionally perform a health check first
	if *healthCheckOnly {
		fmt.Printf("Checking health of %s...\n", *addr)
		err := plugin.GRPCRemoteClientHealthCheck(context.Background(), *addr)
		if err != nil {
			log.Fatalf("Health check failed: %v", err)
		}
		fmt.Println("Server is healthy!")
		return
	}

	// Create a remote gRPC client. This connects to the server over the
	// network with automatic keepalive for reliable connections.
	client, err := plugin.NewGRPCRemoteClient(&plugin.GRPCRemoteClientConfig{
		Addr:    *addr,
		Plugins: shared.PluginMap,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Dispense the KV plugin from the remote server
	raw, err := client.Dispense("kv")
	if err != nil {
		log.Fatalf("Failed to dispense plugin: %v", err)
	}

	kv := raw.(shared.KV)

	// Execute the requested command
	switch args[0] {
	case "get":
		if len(args) < 2 {
			log.Fatal("Usage: client get <key>")
		}
		value, err := kv.Get(args[1])
		if err != nil {
			log.Fatalf("Get failed: %v", err)
		}
		fmt.Println(string(value))

	case "put":
		if len(args) < 3 {
			log.Fatal("Usage: client put <key> <value>")
		}
		err := kv.Put(args[1], []byte(args[2]))
		if err != nil {
			log.Fatalf("Put failed: %v", err)
		}
		fmt.Println("OK")

	default:
		log.Fatalf("Unknown command: %s (use 'get' or 'put')", args[0])
	}
}
