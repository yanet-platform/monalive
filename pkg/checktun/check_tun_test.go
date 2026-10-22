package checktun

import (
	"errors"
	"net/netip"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitSocketsRetainsEncapsulationSockets(t *testing.T) {
	tun := &CheckTun{}
	opened := make(map[[2]int]int)
	err := tun.initSockets(func(domain, proto int) (int, error) {
		fd := len(opened) + 100
		opened[[2]int{domain, proto}] = fd
		return fd, nil
	})
	require.NoError(t, err)
	require.Len(t, opened, 6)

	for _, outer := range []struct {
		ip     string
		domain int
	}{
		{"192.0.2.1", syscall.AF_INET},
		{"2001:db8::1", syscall.AF_INET6},
	} {
		for _, innerVersion := range []int{4, 6} {
			for _, method := range []byte{IPVSConnFTunnel, IPVSConnFGRETunnel} {
				proto := syscall.IPPROTO_GRE
				if method == IPVSConnFTunnel {
					proto = syscall.IPPROTO_IPIP
					if innerVersion == 6 {
						proto = syscall.IPPROTO_IPV6
					}
				}
				fd, err := tun.getSocket(&tunMessage{
					ip:              netip.MustParseAddr(outer.ip),
					protocolVersion: innerVersion,
					lvsMethod:       method,
				})
				require.NoError(t, err)
				require.Equal(t, opened[[2]int{outer.domain, proto}], fd)
			}
		}
	}
}

func TestInitSocketsReturnsOpenError(t *testing.T) {
	tun := &CheckTun{}
	want := errors.New("cannot open socket")
	err := tun.initSockets(func(domain, proto int) (int, error) {
		return -1, want
	})
	require.ErrorIs(t, err, want)
}
