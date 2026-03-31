// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

// Package shared contains shared data between the host and plugins.
// This is used by both the remote plugin server and client.
package shared

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-plugin/examples/grpc/proto"
	"google.golang.org/grpc"
)

// Handshake is a common handshake that is shared by plugin and host.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "BASIC_PLUGIN",
	MagicCookieValue: "hello",
}

// PluginMap is the map of plugins we can dispense.
var PluginMap = map[string]plugin.Plugin{
	"kv": &KVGRPCPlugin{},
}

// KV is the interface that we're exposing as a plugin.
type KV interface {
	Put(key string, value []byte) error
	Get(key string) ([]byte, error)
}

// KVGRPCPlugin is the implementation of plugin.GRPCPlugin for remote serving.
type KVGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	Impl KV
}

func (p *KVGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterKVServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *KVGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: proto.NewKVClient(c)}, nil
}

// GRPCClient is an implementation of KV that talks over gRPC.
type GRPCClient struct {
	client proto.KVClient
}

func (c *GRPCClient) Put(key string, value []byte) error {
	_, err := c.client.Put(context.Background(), &proto.PutRequest{
		Key:   key,
		Value: value,
	})
	return err
}

func (c *GRPCClient) Get(key string) ([]byte, error) {
	resp, err := c.client.Get(context.Background(), &proto.GetRequest{
		Key: key,
	})
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// GRPCServer is the gRPC server that GRPCClient talks to.
type GRPCServer struct {
	proto.UnimplementedKVServer
	Impl KV
}

func (s *GRPCServer) Put(ctx context.Context, req *proto.PutRequest) (*proto.Empty, error) {
	return &proto.Empty{}, s.Impl.Put(req.Key, req.Value)
}

func (s *GRPCServer) Get(ctx context.Context, req *proto.GetRequest) (*proto.GetResponse, error) {
	v, err := s.Impl.Get(req.Key)
	return &proto.GetResponse{Value: v}, err
}

// InMemoryKV is a simple in-memory KV store for demo purposes.
type InMemoryKV struct {
	store map[string][]byte
}

func NewInMemoryKV() *InMemoryKV {
	return &InMemoryKV{store: make(map[string][]byte)}
}

func (kv *InMemoryKV) Put(key string, value []byte) error {
	kv.store[key] = value
	fmt.Printf("PUT %s = %s\n", key, string(value))
	return nil
}

func (kv *InMemoryKV) Get(key string) ([]byte, error) {
	value, ok := kv.store[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	fmt.Printf("GET %s = %s\n", key, string(value))
	return value, nil
}
