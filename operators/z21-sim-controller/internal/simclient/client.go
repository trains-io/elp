package simclient

import (
	"context"
	"fmt"
	"sync"

	controlv1 "github.com/trains-io/elp/apps/z21-sim/api/control/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client applies simulator control-plane RPCs.
type Client interface {
	NotifyCANDeviceDetected(ctx context.Context, req *controlv1.NotifyCANDeviceDetectedRequest) error
	Close() error
}

// GRPCClient dials a z21-sim gRPC control endpoint.
type GRPCClient struct {
	addr string

	mu   sync.Mutex
	conn *grpc.ClientConn
	api  controlv1.SimulatorServiceClient
}

// NewGRPC creates a lazy gRPC client for addr.
func NewGRPC(addr string) *GRPCClient {
	return &GRPCClient{addr: addr}
}

func (c *GRPCClient) NotifyCANDeviceDetected(ctx context.Context, req *controlv1.NotifyCANDeviceDetectedRequest) error {
	api, err := c.client(ctx)
	if err != nil {
		return err
	}
	_, err = api.NotifyCANDeviceDetected(ctx, req)
	return err
}

func (c *GRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.api = nil
	return err
}

func (c *GRPCClient) client(ctx context.Context) (controlv1.SimulatorServiceClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.api != nil {
		return c.api, nil
	}
	conn, err := grpc.NewClient(
		c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial simulator gRPC %s: %w", c.addr, err)
	}
	if ctx.Err() != nil {
		_ = conn.Close()
		return nil, ctx.Err()
	}
	c.conn = conn
	c.api = controlv1.NewSimulatorServiceClient(conn)
	return c.api, nil
}
