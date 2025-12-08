// Package yanet provides a wrapper around the YANET client for interacting with
// the YANET load balancer.
package yanet

import (
	"context"
	"fmt"

	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/types/port"
	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/pkg/yanet"

	yanetpb "github.com/yanet-platform/monalive/gen/yanet/libprotobuf"
)

// Client wraps a YANET client for interacting with the balancer service.
type Client struct {
	client *yanet.Client
}

// NewClient creates a new YANET client instance.
func NewClient(config *Config) (*Client, error) {
	// Create a new YANET client with the specified socket path.
	client, err := yanet.NewClient(yanet.WithControlPlaneSockPath(config.SockPath))
	if err != nil {
		return nil, err
	}
	return &Client{
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
	// Send a request to apply all cached real servers events.
	if _, err := m.client.RealFlush(context.Background(), &yanetpb.Empty{}); err != nil {
		return fmt.Errorf("failed to process the request: %w", err)
	}
	return nil
}

type (
	services = map[key.Service]reals
	reals    = map[key.Real]struct{}
)

// GetState retrieves the current state of the YANET load balancer.
func (m *Client) GetState(ctx context.Context) (map[string]services, error) {
	// Prepare a request to fetch the current state of the real servers.
	req := yanetpb.BalancerRealFindRequest{}
	resp, err := m.client.RealFind(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to process the request: %w", err)
	}

	// Map to hold the state of balancers.
	state := make(map[string]services, len(resp.Balancers))
	for _, balancer := range resp.Balancers {
		// Map to hold services for each balancer.
		servicesState := make(services, len(balancer.Services))
		for _, service := range balancer.Services {
			// Convert protobuf service key to internal key.Service type.
			serviceKey, err := fmtFromProtoService(service.Key)
			if err != nil {
				return nil, err
			}

			// Map to hold real servers for each service.
			realsState := make(reals, len(service.Reals))
			for _, real := range service.Reals {
				// Convert protobuf real server to internal key.Real type.
				realKey, err := fmtFromProtoReal(real)
				if err != nil {
					return nil, err
				}
				realsState[realKey] = struct{}{}
			}
			servicesState[serviceKey] = realsState
		}

		// Add the state of services for each balancer module.
		module := balancer.Module
		state[module] = servicesState
	}

	return state, nil
}

// updateReal updates the status and weight of a real server in the YANET load
// balancer.
func (m *Client) updateReal(ctx context.Context, balancerKey key.Balancer, enable bool, weight weight.Weight) error {
	service := balancerKey.Service
	real := balancerKey.Real

	// Prepare a request to update the real server.
	req := &yanetpb.BalancerRealRequest_Real{
		Module:    "balancer0",
		VirtualIp: fmtToProtoAddr(service.Addr),
		Proto:     fmtToProtoProtocol(service.Proto),
		RealIp:    fmtToProtoAddr(real.Addr),
		Enable:    enable,
	}

	// Fill up all optional fields in the request.
	if p := service.Port; p != port.Omitted {
		req.VirtualPortOpt = &yanetpb.BalancerRealRequest_Real_VirtualPort{VirtualPort: uint32(p.Value())}
	}
	if p := real.Port; p != port.Omitted {
		req.RealPortOpt = &yanetpb.BalancerRealRequest_Real_RealPort{RealPort: uint32(p.Value())}
	}
	if enable {
		req.WeightOpt = &yanetpb.BalancerRealRequest_Real_Weight{Weight: weight.Uint32()}
	}

	// Pack up request to the required format.
	reqs := &yanetpb.BalancerRealRequest{
		Reals: []*yanetpb.BalancerRealRequest_Real{req},
	}

	// Send the request to update the real server in the load balancer.
	if _, err := m.client.Real(ctx, reqs); err != nil {
		return fmt.Errorf("failed to process the request: %w", err)
	}
	return nil
}
