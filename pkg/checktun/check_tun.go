package checktun

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"net/netip"
	"sync"
	"syscall"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/yanet-platform/go-nfqueue/v2"
	"github.com/yanet-platform/netlink"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	IPOptionForExperiment = 30         // [RFC4727]
	IPVSConnFTunnel       = byte(0x02) // matching check tun lite
	IPVSConnFGRETunnel    = byte(0x05)
)

const (
	v4OptionLenForV4 = 7
	v4OptionLenForV6 = 19

	v6OptionLenForV4 = 5
	v6OptionLenForV6 = 17
)

type tunMessage struct {
	ip netip.Addr

	payload         []byte
	protocolVersion int
	lvsMethod       byte
}

func (d *tunMessage) getSockaddr() (syscall.Sockaddr, error) {
	switch {
	case d.ip.Is4():
		return &syscall.SockaddrInet4{Addr: d.ip.As4()}, nil
	case d.ip.Is6():
		return &syscall.SockaddrInet6{Addr: d.ip.As16()}, nil
	}

	return nil, fmt.Errorf("incorrect length")
}

type CheckTun struct {
	nf *nfqueue.Nfqueue

	logger *slog.Logger
	config Config

	IPv4Bind netip.Addr
	IPv6Bind netip.Addr

	readChan    chan []byte
	verdictChan chan uint32

	packetInQueue      int
	packetInQueueMutex sync.Mutex

	sockets map[int]map[int]int
}

func New(config Config, logger *slog.Logger) *CheckTun {
	logger = logger.With(slog.String("event_type", "yanet"))

	readChan := make(chan []byte)
	verdictChan := make(chan uint32)

	return &CheckTun{
		sockets:     make(map[int]map[int]int),
		config:      config,
		logger:      logger,
		readChan:    readChan,
		verdictChan: verdictChan,
	}
}

func (c *CheckTun) Run(ctx context.Context) error {
	config := &nfqueue.Config{
		NfQueue:       c.config.NfQueue,
		MaxPacketLen:  c.config.MaxPacketLen,
		MaxQueueLen:   c.config.MaxQueueLen,
		Copymode:      nfqueue.NfQnlCopyPacket,
		WriteTimeout:  c.config.WriteTimeout,
		WorkerNum:     c.config.WorkerNum,
		ReceiveBuffer: c.config.ReceiveBuffer,
	}

	if ip, err := netip.ParseAddr(c.config.IPv4Bind); err == nil {
		c.IPv4Bind = ip
	}

	if ip, err := netip.ParseAddr(c.config.IPv6Bind); err == nil {
		c.IPv6Bind = ip
	}

	nf, err := nfqueue.Open(config)
	if err != nil {
		return fmt.Errorf("could not open nfqueue socket: %w", err)
	}

	if err := nf.Con.SetOption(netlink.NoENOBUFS, true); err != nil {
		return fmt.Errorf("set option: %w", err)
	}

	if err := nf.Con.SetReadBuffer(c.config.SocketBuffer); err != nil {
		return fmt.Errorf("failed to set read buffer: %w", err)
	}

	if err := c.initSockets(); err != nil {
		return fmt.Errorf("init socket: %w", err)
	}

	err = nf.RegisterWithErrorFunc(ctx, c.nfqueueHandler, func(err error) int {
		if opError, ok := err.(*netlink.OpError); ok {
			if opError.Timeout() || opError.Temporary() {
				return 0
			}
		}

		c.logger.Error("could not receive message", slog.Any("error", err))
		return 0
	})
	if err != nil {
		return fmt.Errorf("register function as callback: %w", err)
	}

	c.nf = nf

	return nil
}

func (c *CheckTun) Stop() {
	if err := c.nf.Close(); err != nil {
		c.logger.Error("check tun close", slog.Any("error", err))
	}
}

func (c *CheckTun) encapsulatePacket(payload []byte) error {
	var message *tunMessage
	version := getProtocolVersion(payload)

	switch version {
	case ipv4.Version:
		message = ipv4Handler(payload)
	case ipv6.Version:
		message = ipv6Handler(payload)
	}

	if message == nil {
		return nil
	}

	socket, err := c.getSocket(message)
	if err != nil {
		return fmt.Errorf("failed to get socket: %w", err)
	}

	err = sendMessage(socket, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (c *CheckTun) getSocket(message *tunMessage) (int, error) {
	var domain, proto int

	switch {
	case message.ip.Is4():
		domain = syscall.AF_INET
	case message.ip.Is6():
		domain = syscall.AF_INET6
	default:
		return 0, fmt.Errorf("invalid ip")
	}

	switch message.lvsMethod {
	case IPVSConnFTunnel:
		switch message.protocolVersion {
		case ipv4.Version:
			proto = syscall.IPPROTO_IPIP
		case ipv6.Version:
			proto = syscall.IPPROTO_IPV6
		default:
			return 0, fmt.Errorf("invalide proto version: %d", message.protocolVersion)
		}
	case IPVSConnFGRETunnel:
		proto = syscall.IPPROTO_GRE
	default:
		return 0, fmt.Errorf("unknown lvs method")
	}

	if socket, ok := c.sockets[domain][proto]; ok {
		return socket, nil
	}

	return 0, fmt.Errorf("udefined socket")
}

func (c *CheckTun) initSockets() error {
	domains := []int{syscall.AF_INET, syscall.AF_INET6}
	protos := []int{syscall.IPPROTO_IPIP, syscall.IPPROTO_IPV6, syscall.IPPROTO_GRE}

	for _, domain := range domains {
		for _, proto := range protos {
			fd, err := c.newSocket(domain, proto)
			if err != nil {
				return fmt.Errorf("new socket: %w", err)
			}

			if _, ok := c.sockets[domain]; !ok {
				c.sockets[domain] = make(map[int]int)
			}

			c.sockets[domain][proto] = fd
		}
	}

	return nil
}

func (c *CheckTun) newSocket(domain, proto int) (int, error) {
	fd, err := syscall.Socket(domain, syscall.SOCK_RAW, proto)
	if err != nil {
		return 0, fmt.Errorf("init socket: %w", err)
	}

	var addr syscall.Sockaddr
	if domain == syscall.AF_INET && c.IPv4Bind.Is4() {
		addr = &syscall.SockaddrInet4{Addr: c.IPv4Bind.As4()}
	} else if domain == syscall.AF_INET6 && c.IPv6Bind.Is6() {
		addr = &syscall.SockaddrInet6{Addr: c.IPv6Bind.As16()}
	}

	if addr != nil {
		err := syscall.Bind(fd, addr)
		if err != nil {
			return 0, fmt.Errorf("bind ip error")
		}
	}

	return fd, nil
}

func sendMessage(socket int, message *tunMessage) error {
	addr, err := message.getSockaddr()
	if err != nil {
		return fmt.Errorf("convert to sockaddr: %w", err)
	}

	switch message.lvsMethod {
	case IPVSConnFTunnel:
		if err := syscall.Sendto(socket, message.payload, 0, addr); err != nil {
			return fmt.Errorf("sendto failed: %w", err)
		}
	case IPVSConnFGRETunnel:
		var ethernetType layers.EthernetType
		switch message.protocolVersion {
		case ipv4.Version:
			ethernetType = layers.EthernetTypeIPv4
		case ipv6.Version:
			ethernetType = layers.EthernetTypeIPv6
		}

		gre := &layers.GRE{Protocol: ethernetType}
		pbuf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		err = gopacket.SerializeLayers(pbuf, opts, gre, gopacket.Payload(message.payload))
		if err != nil {
			return fmt.Errorf("serialize failed: %w", err)
		}

		if err := syscall.Sendto(socket, pbuf.Bytes(), 0, addr); err != nil {
			return fmt.Errorf("sendto failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown lvs method")
	}

	return nil
}

func (c *CheckTun) dropPacket(packetID uint32) error {
	c.packetInQueueMutex.Lock()

	c.packetInQueue++
	if c.packetInQueue < 100 {
		c.packetInQueueMutex.Unlock()
		return nil
	}

	c.packetInQueue = 0
	c.packetInQueueMutex.Unlock()

	for {
		err := c.nf.SetVerdictBatch(packetID, nfqueue.NfDrop)
		if err == nil {
			break
		}

		if opError, ok := err.(*netlink.OpError); ok {
			if !opError.Timeout() && !opError.Temporary() {
				return fmt.Errorf("failed to set verdict: %w", err)
			}
		} else {
			return fmt.Errorf("failed to set verdict: %w", err)
		}
	}

	return nil
}

func (c *CheckTun) nfqueueHandler(a nfqueue.Attribute) int {
	if a.PacketID == nil {
		return 0
	}

	if err := c.dropPacket(*a.PacketID); err != nil {
		c.logger.Error("failed to drop packet", slog.Any("error", err))
	}

	if a.Mark != nil && *a.Mark == math.MaxUint32 {
		if a.Payload == nil || len(*a.Payload) < 1 {
			return 0
		}

		if err := c.encapsulatePacket(*a.Payload); err != nil {
			c.logger.Error("failed to encapsulate packet", slog.Any("error", err))
		}
	}

	return 0
}

func getProtocolVersion(payload []byte) int {
	return int(payload[0] >> 4)
}

func getIpv4HeaderLen(payload []byte) int {
	return int(payload[0]&0x0f) << 2
}

func ipv4Handler(payload []byte) *tunMessage {
	headerLen := getIpv4HeaderLen(payload)
	if len(payload) < headerLen || headerLen < ipv4.HeaderLen {
		return nil
	}

	options := payload[ipv4.HeaderLen:headerLen]

	for i, minHeaderLen := 0, 2; i <= len(options)-minHeaderLen; i += int(options[i+1]) {
		optionLen := int(options[i+1])
		if optionLen == 0 || optionLen+i > len(options) {
			break
		}

		optionType := options[i]
		if optionType != IPOptionForExperiment {
			continue
		}

		if optionLen != v4OptionLenForV4 && optionLen != v4OptionLenForV6 {
			continue
		}

		ip, ok := netip.AddrFromSlice(options[i+2 : i+optionLen-1])
		if !ok {
			continue
		}

		lvsMethod := options[i+optionLen-1]

		removeIpv4Options(&payload)

		message := &tunMessage{
			ip:              ip,
			payload:         payload,
			lvsMethod:       lvsMethod,
			protocolVersion: ipv4.Version,
		}

		return message
	}

	return nil
}

func removeIpv4Options(payloadPtr *[]byte) {
	payload := *payloadPtr

	// remove all options
	headerLen := getIpv4HeaderLen(payload)
	copy(payload[ipv4.HeaderLen:], payload[headerLen:])

	delta := headerLen - ipv4.HeaderLen
	totalLen := len(payload) - delta
	*payloadPtr = payload[:totalLen]

	payload[0] = ipv4.Version<<4 + 5
	binary.BigEndian.PutUint16(payload[2:4], uint16(totalLen))

	// fix checksum
	binary.BigEndian.PutUint16(payload[10:12], 0)
	checksum := calculateIpv4Checksum(payload[:ipv4.HeaderLen])
	binary.BigEndian.PutUint16(payload[10:12], checksum)
}

func ipv6Handler(payload []byte) *tunMessage {
	if len(payload) < ipv6.HeaderLen {
		return nil
	}

	payloadLen := int(binary.BigEndian.Uint16(payload[4:6]))
	options := payload[ipv6.HeaderLen : ipv6.HeaderLen+payloadLen]

	headerType := &payload[6]
	var octetNum byte

	for i, minHeaderLen := 0, 8; i <= len(options)-minHeaderLen; i += int(octetNum+1) << 3 {
		switch *headerType {
		case syscall.IPPROTO_DSTOPTS:
			octetNum = options[i+1]

			optionType := options[i+2]
			if optionType != IPOptionForExperiment {
				break
			}

			optionLen := int(options[i+3])
			if optionLen+i > len(options) {
				break
			}

			if optionLen != v6OptionLenForV4 && optionLen != v6OptionLenForV6 {
				continue
			}

			ip, ok := netip.AddrFromSlice(options[i+4 : i+4+optionLen-1])
			if !ok {
				continue
			}

			lvsMethod := options[i+4+optionLen-1]

			// remove whole option
			*headerType = options[i]
			delta := (int(octetNum+1) << 3)

			binary.BigEndian.PutUint16(payload[4:6], uint16(payloadLen-delta))
			copy(payload[ipv6.HeaderLen+i:], payload[ipv6.HeaderLen+i+delta:])

			totalLen := len(payload) - delta
			payload = payload[:totalLen]

			message := &tunMessage{
				ip:              ip,
				payload:         payload,
				lvsMethod:       lvsMethod,
				protocolVersion: ipv6.Version,
			}

			return message
		case syscall.IPPROTO_FRAGMENT:
			octetNum = 1
		case syscall.IPPROTO_HOPOPTS, syscall.IPPROTO_ROUTING:
			octetNum = options[i+1]
		default:
			return nil
		}

		headerType = &options[i]
	}

	return nil
}

func calculateIpv4Checksum(payload []byte) uint16 {
	sum := 0

	for i := 0; i < len(payload)-1; i += 2 {
		word := int(payload[i])<<8 | int(payload[i+1])
		sum += word
	}

	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}

	checksum := ^sum
	return uint16(checksum)
}
