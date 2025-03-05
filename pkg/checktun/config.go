package checktun

import "time"

type Config struct {
	Enabled       bool          `yaml:"run_check_tun"`
	NfQueue       uint16        `yaml:"nfqueue"`
	MaxQueueLen   uint32        `yaml:"max_queue_len"`
	MaxPacketLen  uint32        `yaml:"max_packet_len"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	IPv4Bind      string        `yaml:"ipv4_bind"`
	IPv6Bind      string        `yaml:"ipv6_bind"`
	WorkerNum     int           `yaml:"worker_num"`
	SocketBuffer  int           `yaml:"socket_buffer"`
	ReceiveBuffer int           `yaml:"receive_buffer"`
}
