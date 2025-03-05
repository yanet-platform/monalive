package check

import (
	"context"
	"net"
	"net/netip"

	"monalive/internal/types/weight"
	"monalive/internal/utils/xnet"
)

// TCPCheck performs TCP connectivity checks based on the provided
// configuration.
type TCPCheck struct {
	config Config     // configuration for the TCP check
	uri    string     // URI for the TCP connection
	dialer net.Dialer // dialer for establishing TCP connections
}

// NewTCPCheck creates a new instance of TCPCheck.
func NewTCPCheck(config Config, forwardingData xnet.ForwardingData) *TCPCheck {
	check := &TCPCheck{
		config: config,
	}
	check.uri = check.URI()
	check.dialer = xnet.NewDialer(config.BindIP, config.GetConnectTimeout(), forwardingData)

	return check
}

// Do performs the TCP check. It attempts to establish a TCP connection to the
// configured URI. If successful, it sets the Metadata to indicate that the
// connection is alive and assigns an ommited weight since TCP check does not
// support dynamic weight by design. If the connection fails, it marks the
// metadata inactive. Returns an error if the connection fails or if there is an
// issue closing the socket.
func (m *TCPCheck) Do(ctx context.Context, md *Metadata) (err error) {
	defer func() {
		if err != nil {
			// Mark the metadata inactive if an error has occurred.
			md.SetInactive()
		}
	}()

	sock, err := m.dialer.DialContext(ctx, "tcp", m.uri)
	if err != nil {
		return err
	}

	if err := sock.Close(); err != nil {
		return err
	}

	// Update metadata to indicate the connection is alive.
	md.Alive = true
	md.Weight = weight.Omitted

	return nil
}

// URI returns the URI for the TCP connection based on the configuration. It
// formats the IP address and port from the configuration into a string suitable
// for use with the dialer.
func (m *TCPCheck) URI() string {
	if m.uri != "" {
		// Return the precomputed URI if available.
		return m.uri
	}

	return netip.AddrPortFrom(m.config.ConnectIP, m.config.ConnectPort.Value()).String()
}
