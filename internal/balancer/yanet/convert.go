package yanet

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/types/port"

	yanetpb "github.com/yanet-platform/monalive/gen/yanet/libprotobuf"
)

// fmtToProtoAddr converts a [netip.Addr] to a [yanetpb.IPAddr].
func fmtToProtoAddr(addr netip.Addr) *yanetpb.IPAddr {
	if addr.Is4() {
		// Convert IPv4 address to a protobuf IPAddr.
		return &yanetpb.IPAddr{Addr: &yanetpb.IPAddr_Ipv4{Ipv4: binary.BigEndian.Uint32(addr.AsSlice())}}
	}
	// Convert IPv6 address to a protobuf IPAddr.
	return &yanetpb.IPAddr{Addr: &yanetpb.IPAddr_Ipv6{Ipv6: addr.AsSlice()}}
}

// fmtFromProtoAddr converts a [yanetpb.IPAddr] to a [netip.Addr].
func fmtFromProtoAddr(addr *yanetpb.IPAddr) (netip.Addr, bool) {
	if bytes := addr.GetIpv6(); bytes != nil {
		return netip.AddrFromSlice(bytes)
	}

	protoIPv4 := addr.GetIpv4()
	ipv4 := make(net.IP, net.IPv4len)
	binary.BigEndian.PutUint32(ipv4, protoIPv4)

	return netip.AddrFromSlice(ipv4)
}

// fmtFromProtoService converts a protobuf [yanetpb.ServiceKey] to a [key.Service].
func fmtFromProtoService(s *yanetpb.BalancerRealFindResponse_ServiceKey) (key.Service, error) {
	ip, ok := fmtFromProtoAddr(s.Ip)
	if !ok {
		return key.Service{}, fmt.Errorf("protoServiceKeyConvert: can't parse ip to netip, ip: %s", s.Ip.String())
	}

	var p port.Port
	switch portOpt := s.PortOpt.(type) {
	case *yanetpb.BalancerRealFindResponse_ServiceKey_Port:
		p = port.Port(portOpt.Port)
	default:
		p = port.Omitted
	}

	return key.Service{
		Addr:  ip,
		Port:  p,
		Proto: strings.ToUpper(s.Proto.String()),
	}, nil
}

// fmtFromProtoReal converts a [yanetpb.Real] to a [key.Real].
func fmtFromProtoReal(r *yanetpb.BalancerRealFindResponse_Real) (key.Real, error) {
	ip, ok := fmtFromProtoAddr(r.Ip)
	if !ok {
		return key.Real{}, fmt.Errorf("protoRealConvert: can't parse yanetpb.IPAddr to netip, value: %s", r.Ip.String())
	}

	var p port.Port
	switch portOpt := r.PortOpt.(type) {
	case *yanetpb.BalancerRealFindResponse_Real_Port:
		p = port.Port(portOpt.Port)
	default:
		p = port.Omitted
	}

	return key.Real{
		Addr: ip,
		Port: p,
	}, nil
}

// fmtToProtoProtocol converts a protocol string to a [yanetpb.NetProto].
func fmtToProtoProtocol(protocol string) yanetpb.NetProto {
	switch protocol {
	case "TCP":
		return yanetpb.NetProto_tcp
	case "UDP":
		return yanetpb.NetProto_udp
	default:
		return yanetpb.NetProto_undefined
	}
}
