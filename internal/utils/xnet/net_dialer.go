// Package xnet provides network utilities for setting up custom dialers with
// specific IP forwarding methods and connection settings.
package xnet

import (
	"math"
	"net"
	"net/netip"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/yanet-platform/monalive/pkg/checktun"
)

const (
	// HopLimit specifies the maximum number of hops (TTL) for the IP packet.
	HopLimit = 2

	// FWMark is the firewall mark to be set on the socket.
	FWMark = math.MaxUint32
)

// linger defines the socket linger option, which determines the behavior of the
// socket when it is closed.
var linger = syscall.Linger{
	Onoff:  1, // linger active
	Linger: 0, // close the socket immediately
}

// ForwardingData holds information about the destination for packet tunneling
// and the method to be used for forwarding.
type ForwardingData struct {
	// RealIP is the address of the real server with which the tunnel will be
	// established.
	RealIP netip.Addr
	// ForwardingMethod specifies the method for tunneling the packet.
	ForwardingMethod string
}

// NewDialer creates a new net.Dialer with custom connection settings. The
// dialer sets socket options such as hop limit, IP headers, and firewall marks
// based on the provided ForwardingData. It returns a configured net.Dialer
// instance.
//
// Parameters:
//   - bindIP: The IP address to bind the local end of the connection to.
//   - connTimeout: The timeout duration for establishing the connection.
//   - forwardingData: Contains the real destination IP and the forwarding method.
func NewDialer(bindIP netip.Addr, connTimeout time.Duration, forwardingData ForwardingData) net.Dialer {
	// By default, use IP encapsulation; also supports GRE tunneling.
	forwardingMethod := checktun.IPVSConnFTunnel
	if forwardingData.ForwardingMethod == "GRE" {
		forwardingMethod = checktun.IPVSConnFGRETunnel
	}

	// Convert RealIP to a byte slice for use in header construction.
	realIP := forwardingData.RealIP.AsSlice()

	return net.Dialer{
		Control: func(network, address string, conn syscall.RawConn) error {
			var opterr error
			err := conn.Control(func(fd uintptr) {
				var level, hopsOpt, headerOpt int
				var header string

				// Determine whether the address is IPv4 or IPv6 and set socket
				// options accordingly.
				if strings.Contains(address, ".") {
					level = syscall.SOL_IP
					hopsOpt = syscall.IP_TTL
					headerOpt = syscall.IP_OPTIONS

					// Construct the experimental header for IPv4.
					h := checktun.ConstructExperimentalHeaderWithIP(realIP, forwardingMethod)
					header = unsafe.String(&h[0], len(h))
				} else {
					level = syscall.SOL_IPV6
					hopsOpt = syscall.IPV6_UNICAST_HOPS
					headerOpt = syscall.IPV6_DSTOPTS

					// Construct the destination options header for IPv6.
					h := checktun.ConstructDstHeaderWithIP(realIP, forwardingMethod)
					header = unsafe.String(&h[0], len(h))
				}

				// Set the hop limit for the socket.
				if opterr = syscall.SetsockoptInt(
					int(fd),
					level,
					hopsOpt,
					HopLimit,
				); opterr != nil {
					return
				}

				// Set the IP options or destination options header for the
				// socket.
				if opterr = syscall.SetsockoptString(
					int(fd),
					level,
					headerOpt,
					header,
				); opterr != nil {
					return
				}

				// Set the firewall mark for the socket.
				if opterr = syscall.SetsockoptInt(
					int(fd),
					syscall.SOL_SOCKET,
					syscall.SO_MARK,
					FWMark,
				); opterr != nil {
					return
				}

				// Set the linger option for the socket.
				_ = syscall.SetsockoptLinger(
					int(fd),
					syscall.SOL_SOCKET,
					syscall.SO_LINGER,
					&linger,
				)
			})
			if err != nil {
				return err
			}

			return opterr
		},
		Timeout: connTimeout,
		LocalAddr: &net.TCPAddr{
			IP: bindIP.AsSlice(),
		},
	}

}
