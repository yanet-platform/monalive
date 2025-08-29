package check

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/utils/xnet"
	"github.com/yanet-platform/monalive/internal/utils/xtls"
)

// GRPCCheck performs gRPC health checks based on the provided configuration.
type GRPCCheck struct {
	config Config     // configuration for the gRPC check
	uri    string     // URI for the gRPC service
	dialer net.Dialer // dialer for creating network connections
}

// NewGRPCCheck creates a new instance of GRPCCheck.
func NewGRPCCheck(config Config, forwardingData xnet.ForwardingData) *GRPCCheck {
	check := &GRPCCheck{
		config: config,
	}
	check.uri = check.URI()
	check.dialer = xnet.NewDialer(config.BindIP, config.GetConnectTimeout(), forwardingData)

	return check
}

// Do performs the gRPC health check by creating a connection, sending a health
// check request, and handling the response. It updates the Metadata based on
// the response or marks it inactive if an error has occurred.
func (m *GRPCCheck) Do(ctx context.Context, md *Metadata) (err error) {
	defer func() {
		if err != nil {
			// Mark the metadata inactive if an error has occurred.
			md.SetInactive()
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, m.config.GetCheckTimeout())
	defer cancel()

	// Setup connection.
	conn, err := m.newConn(ctx)
	if err != nil {
		return fmt.Errorf("failed to create new conn: %w", err)
	}
	defer conn.Close()

	// Create a new health client.
	client := healthpb.NewHealthClient(conn)

	aliveStatus := "0"
	if md.Alive {
		aliveStatus = "1"
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "X-RS-Alive", aliveStatus)

	// Add weight header if dynamic weight is enabled.
	if m.config.DynamicWeight {
		ctx = metadata.AppendToOutgoingContext(ctx, "X-RS-Weight", md.Weight.String())
	}

	var serviceName string
	if m.config.Virtualhost != nil {
		// Use virtual host as name of the service if configured.
		serviceName = *m.config.Virtualhost
	}

	var header metadata.MD
	response, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: serviceName}, grpc.Header(&header))
	if err != nil {
		return err
	}

	// Handle the response and update metadata.
	return m.handle(md, response, header)
}

// URI returns the URI for the gRPC connection based on the configuration. It
// formats the IP address and port from the configuration into a string suitable
// for use with gRPC.
func (m *GRPCCheck) URI() string {
	if m.uri != "" {
		// Return the precomputed URI if available.
		return m.uri
	}

	return netip.AddrPortFrom(m.config.ConnectIP, m.config.ConnectPort.Value()).String()
}

// newConn creates a new gRPC connection with the provided context. It sets up
// the transport credentials, user agent, and context dialer for the connection.
func (m *GRPCCheck) newConn(ctx context.Context) (*grpc.ClientConn, error) {
	return grpc.DialContext(
		ctx,
		m.uri,
		grpc.WithTransportCredentials(credentials.NewTLS(xtls.TLSConfig())), // use TLS credentials
		grpc.WithUserAgent(UserAgentRequestHeader),                          // set the user agent
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return m.dialer.DialContext(ctx, "tcp", addr) // use custom dialer
		}),
	)
}

// handle processes the gRPC health check response. It validates the status and
// updates the Metadata with the response details.
func (m *GRPCCheck) handle(md *Metadata, response *healthpb.HealthCheckResponse, header metadata.MD) error {
	if status := response.GetStatus(); !m.matchStatus(status) {
		// Return error if status mismatch.
		return fmt.Errorf("status does not match: %s", status.String())
	}

	// Update metadata to indicate the connection is alive.
	md.Alive = true
	// Update metadata with the weight from response.
	md.Weight = m.getWeightFrom(header)

	return nil
}

// matchStatus checks if the response status matches the expected status.
func (m *GRPCCheck) matchStatus(status healthpb.HealthCheckResponse_ServingStatus) bool {
	return status == healthpb.HealthCheckResponse_SERVING
}

// getWeightFrom extracts the weight from the gRPC response metadata based on
// the configured dynamic weight settings.
func (m *GRPCCheck) getWeightFrom(header metadata.MD) weight.Weight {
	if !m.config.DynamicWeight {
		// Return omitted if dynamic weight is not enabled.
		return weight.Omitted
	}

	values := header.Get("X-RS-Weight")
	if len(values) != 1 {
		// Return omitted if weight header is missing or invalid.
		return weight.Omitted
	}

	var weight weight.Weight
	// Attempt to unmarshal weight from header.
	_ = weight.UnmarshalText([]byte(values[0]))

	return weight
}
