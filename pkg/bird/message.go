package bird

import (
	"net/netip"
)

// PrefixStatus represents the status of a network prefix, either enabled or
// disabled.
type PrefixStatus byte

const (
	// Enable indicates that the prefix should be enabled.
	Enable PrefixStatus = 0x01

	// Disable indicates that the prefix should be disabled.
	Disable PrefixStatus = 0x00
)

const (
	ipv4 byte = 0x04 // ipv4 indicates that the message contains an IPv4 address
	ipv6 byte = 0x06 // ipv6 indicates that the message contains an IPv6 address
)

// Message represents a message sent to the BIRD daemon. It contains information
// about the IP version, address, prefix length, and status.
type Message struct {
	ipVer   byte     // IP version: either ipv4 or ipv6
	ipAddr  [16]byte // the IP address associated with the prefix
	prefLen byte     // the length of the prefix (number of significant bits)
	state   byte     // the status of the prefix (enabled or disabled)
}

// NewMessage creates a new Message with the given network prefix and status. It
// automatically determines the IP version based on the prefix and sets the
// appropriate fields.
func NewMessage(prefix netip.Prefix, status PrefixStatus) Message {
	ipVer := ipv4
	if prefix.Addr().Is6() {
		ipVer = ipv6
	}

	ipAddr := prefix.Addr().As16()
	prefLen := prefix.Bits()
	if prefLen == -1 {
		// netip.Bits() returns -1 for invalide prefix length. In this case,
		// prefix treated as an address, so it is assigned the value of the
		// address bit length.
		prefLen = prefix.Addr().BitLen()
	}

	return Message{
		ipVer:   ipVer,
		ipAddr:  ipAddr,
		prefLen: byte(prefLen),
		state:   byte(status),
	}
}
