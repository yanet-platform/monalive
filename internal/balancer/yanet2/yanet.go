// Package yanet2 provides a wrapper around the YANET2 client for interacting
// with the YANET2 load balancer module.
package yanet2

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/yanet-platform/monalive/gen/yanet2/common/commonpb"
	"github.com/yanet-platform/monalive/gen/yanet2/modules/balancer/controlplane/balancerpb"
	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/types/weight"
)

// Client wraps a YANET client for interacting with the balancer service.
type Client struct {
	config *Config
	client balancerpb.BalancerServiceClient
}

// NewClient creates a new YANET2 balancer module client instance.
func NewClient(config *Config) (*Client, error) {
	grpcClient, err := grpc.NewClient(config.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %w", err)
	}

	client := balancerpb.NewBalancerServiceClient(grpcClient)

	return &Client{
		config: config,
		client: client,
	}, nil
}

// EnableReal activates a real server in the YANET load balancer with the
// specified weight.
func (m *Client) EnableReal(ctx context.Context, balancerKey key.Balancer, weight weight.Weight) error {
	// Update the real server to enable it with the specified weight.
	return m.updateReal(ctx, balancerKey, true, weight)
}

// DisableReal deactivates a real server in the YANET load balancer.
func (m *Client) DisableReal(ctx context.Context, balancerKey key.Balancer) error {
	// Update the real server to disable it. Weight is ignored in disable
	// request.
	return m.updateReal(ctx, balancerKey, false, weight.Omitted)
}

// Flush applies all cached events.
func (m *Client) Flush(ctx context.Context) error {
	m.client.FlushRealUpdates(ctx, &balancerpb.FlushRealUpdatesRequest{
		Target: &commonpb.TargetModule{
			ConfigName:        m.config.BalancerModuleName,
			DataplaneInstance: m.config.DataplaneInstance,
		},
	})
	return nil
}

// updateReal updates the status and weight of a real server in the YANET load
// balancer.
func (m *Client) updateReal(ctx context.Context, balancerKey key.Balancer, enable bool, weight weight.Weight) error {
	service := balancerKey.Service
	real := balancerKey.Real

	var proto balancerpb.TransportProto
	switch service.Proto {
	case "TCP":
		proto = balancerpb.TransportProto_TCP
	case "UPD":
		proto = balancerpb.TransportProto_UDP
	default:
		return fmt.Errorf("unsupported protocol: %s", service.Proto)
	}

	update := &balancerpb.RealUpdate{
		VirtualIp: service.Addr.AsSlice(),
		Proto:     proto,
		Port:      uint32(service.Port.Value()),
		RealIp:    real.Addr.AsSlice(),
		Enable:    enable,
		Weight:    weight.Uint32(),
	}

	req := &balancerpb.UpdateRealsRequest{
		Target: &commonpb.TargetModule{
			ConfigName:        m.config.BalancerModuleName,
			DataplaneInstance: m.config.DataplaneInstance,
		},
		Updates: []*balancerpb.RealUpdate{update},
		Buffer:  true,
	}
	// Send the request to update the real server in the load balancer.
	if _, err := m.client.UpdateReals(ctx, req); err != nil {
		return fmt.Errorf("failed to process the request: %w", err)
	}

	return nil
}
