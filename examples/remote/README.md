# Remote Plugin Example

This example demonstrates how to use go-plugin for remote gRPC plugin
communication over the network. Unlike the local subprocess-based examples,
this shows how to connect to a plugin server that may be running on a
different machine.

## Features

- **Network Transport**: Plugins communicate over TCP/IP instead of local sockets
- **Keepalive**: Automatic keepalive pings maintain reliable connections
- **Health Checks**: Clients can verify server availability before connecting
- **Graceful Shutdown**: Server handles SIGTERM for clean shutdown

## Running the Example

### 1. Start the Server

```bash
cd server
go run . -addr :50051
```

The server will listen on port 50051 and accept connections from anywhere.

### 2. Run the Client

In another terminal:

```bash
cd client

# Put a value
go run . -addr localhost:50051 put mykey "hello world"

# Get the value
go run . -addr localhost:50051 get mykey

# Health check
go run . -addr localhost:50051 -health
```

## Code Structure

- `shared/` - Shared interface and gRPC implementation
- `server/` - Remote plugin server
- `client/` - Remote plugin client

## Key Differences from Local Plugins

1. **No Subprocess Management**: The server runs as a standalone process
2. **Network Addresses**: Uses `host:port` addresses instead of local sockets
3. **Keepalive**: Built-in keepalive for reliable network transport
4. **TLS Support**: Can use TLS for secure connections over untrusted networks

## API Usage

### Server Side

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "github.com/hashicorp/go-plugin"
)

// Create server with keepalive
grpcServer := grpc.NewServer(
    grpc.KeepaliveEnforcementPolicy(
        plugin.DefaultGRPCServerKeepaliveEnforcementPolicy(),
    ),
)

// Register health check
healthCheck := health.NewServer()
healthCheck.SetServingStatus(plugin.GRPCServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
grpc_health_v1.RegisterHealthServer(grpcServer, healthCheck)

// Register plugins and serve
grpcServer.Serve(listener)
```

### Client Side

```go
import "github.com/hashicorp/go-plugin"

// Connect to remote server with keepalive
client, err := plugin.NewGRPCRemoteClient(&plugin.GRPCRemoteClientConfig{
    Addr:    "localhost:50051",
    Plugins: pluginMap,
})
defer client.Close()

// Use the plugin
raw, err := client.Dispense("kv")
kv := raw.(KV)
```
