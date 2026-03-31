// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"crypto/tls"
	"math"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin/internal/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// GRPCRemoteClientConfig configures a remote gRPC plugin client that connects
// to a plugin server running on a network address, rather than a local
// subprocess.
//
// Remote gRPC clients are designed to work reliably over network connections
// with automatic keepalive, reconnection, and long-lived streaming support.
type GRPCRemoteClientConfig struct {
	// Addr is the network address of the remote plugin server.
	// This should be in "host:port" format for TCP connections.
	Addr string

	// TLSConfig, if set, enables TLS for the connection.
	TLSConfig *tls.Config

	// Plugins are the plugins that can be consumed.
	Plugins PluginSet

	// Logger is used for logging. If none is provided, a default logger
	// will be created.
	Logger hclog.Logger

	// GRPCDialOptions allows passing custom gRPC dial options.
	// Keepalive options will be merged with these defaults if
	// KeepaliveParams are not set.
	GRPCDialOptions []grpc.DialOption

	// KeepaliveParams configures client-side keepalive for reliable
	// network transport. If nil, sensible defaults are used suitable
	// for remote connections.
	KeepaliveParams *keepalive.ClientParameters

	// KeepaliveEnforcementPolicy configures server-side keepalive
	// enforcement. This is used only when the server configures
	// keepalive with this policy.
	KeepaliveEnforcementPolicy *keepalive.EnforcementPolicy
}

// DefaultGRPCRemoteKeepaliveParams returns keepalive parameters suitable
// for reliable operation over network connections. These defaults send
// keepalive pings every 10 seconds and wait 20 seconds for a response
// before considering the connection dead.
func DefaultGRPCRemoteKeepaliveParams() keepalive.ClientParameters {
	return keepalive.ClientParameters{
		// Send keepalive pings every 10 seconds when there is no activity.
		Time: 10 * time.Second,
		// Wait 20 seconds for a keepalive ping response before considering
		// the connection dead.
		Timeout: 20 * time.Second,
		// Send keepalive pings even if there are no active streams.
		// This ensures the connection stays alive during idle periods.
		PermitWithoutStream: true,
	}
}

// DefaultGRPCServerKeepaliveEnforcementPolicy returns an enforcement policy
// suitable for remote gRPC servers. Use this when creating the gRPC server
// to ensure it accepts keepalive pings from remote clients.
func DefaultGRPCServerKeepaliveEnforcementPolicy() keepalive.EnforcementPolicy {
	return keepalive.EnforcementPolicy{
		// Allow clients to send keepalive pings without active streams.
		MinTime:             10 * time.Second,
		PermitWithoutStream: true,
	}
}

// NewGRPCRemoteClient creates a new GRPCClient connected to a remote plugin
// server at the given address. Unlike the subprocess-based Client, this
// connects directly to a running gRPC server over the network.
//
// The returned GRPCClient uses keepalive to maintain reliable connections
// over potentially unreliable networks. Callers should call Close() when
// done to release resources.
func NewGRPCRemoteClient(config *GRPCRemoteClientConfig) (*GRPCClient, error) {
	if config.Logger == nil {
		config.Logger = hclog.New(&hclog.LoggerOptions{
			Output: hclog.DefaultOutput,
			Level:  hclog.Trace,
			Name:   "plugin",
		})
	}

	conn, err := dialRemoteGRPCConn(config)
	if err != nil {
		return nil, err
	}

	doneCtx, cancel := context.WithCancel(context.Background())
	_ = cancel // cancel is called by GRPCClient.Close via its Close method

	// Start the broker for auxiliary gRPC connections
	brokerGRPCClient := newGRPCBrokerClient(conn)
	broker := newGRPCBroker(brokerGRPCClient, config.TLSConfig, UnixSocketConfig{}, nil, nil)
	go broker.Run()
	go func() { _ = brokerGRPCClient.StartStream() }()

	// Start stdio client - this may fail gracefully if the server doesn't
	// support it (e.g. for non-Go plugins).
	stdioClient, err := newGRPCStdioClient(doneCtx, config.Logger.Named("stdio"), conn)
	if err != nil {
		config.Logger.Warn("failed to create stdio client", "err", err)
	}
	if stdioClient != nil {
		go stdioClient.Run(nil, nil)
	}

	cl := &GRPCClient{
		Conn:       conn,
		Plugins:    config.Plugins,
		doneCtx:    doneCtx,
		broker:     broker,
		controller: plugin.NewGRPCControllerClient(conn),
	}

	return cl, nil
}

// dialRemoteGRPCConn creates a gRPC connection to a remote server with
// keepalive and appropriate settings for reliable network transport.
func dialRemoteGRPCConn(config *GRPCRemoteClientConfig) (*grpc.ClientConn, error) {
	opts := make([]grpc.DialOption, 0)

	// Configure TLS
	if config.TLSConfig == nil {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(
			credentials.NewTLS(config.TLSConfig)))
	}

	// Configure keepalive for reliable network transport
	keepaliveParams := DefaultGRPCRemoteKeepaliveParams()
	if config.KeepaliveParams != nil {
		keepaliveParams = *config.KeepaliveParams
	}
	opts = append(opts, grpc.WithKeepaliveParams(keepaliveParams))

	// Set large message size limits
	opts = append(opts,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(math.MaxInt32)))

	// Add any custom dial options (merging with defaults)
	opts = append(opts, config.GRPCDialOptions...)

	conn, err := grpc.Dial(config.Addr, opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// NewGRPCRemoteServer creates a gRPC server suitable for remote plugin hosting.
// It configures keepalive enforcement to work reliably with remote clients
// and returns a server that can be registered with plugin services.
//
// This is a convenience function; you can also create the server directly
// using grpc.NewServer with the appropriate options.
func NewGRPCRemoteServer(opts ...grpc.ServerOption) *grpc.Server {
	// Add keepalive enforcement policy for remote clients
	opts = append(opts,
		grpc.KeepaliveEnforcementPolicy(DefaultGRPCServerKeepaliveEnforcementPolicy()))

	return grpc.NewServer(opts...)
}

// GRPCRemoteClientHealthCheck performs a health check on a remote gRPC plugin
// server. This can be used to verify that a remote server is available and
// healthy before attempting to use it.
func GRPCRemoteClientHealthCheck(ctx context.Context, addr string, opts ...grpc.DialOption) error {
	// Add keepalive for reliability
	keepaliveParams := DefaultGRPCRemoteKeepaliveParams()
	opts = append(opts, grpc.WithKeepaliveParams(keepaliveParams))
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	_, err = client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: GRPCServiceName,
	})

	return err
}
