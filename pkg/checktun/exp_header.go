package checktun

import (
	"bytes"
	"net"
)

// For ipv4
func ConstructExperimentalHeaderWithIP(ipForEncode net.IP, lvsMethod byte) []byte {
	// type-length-ip-lvs_method option
	r := make([]byte, 2+len(ipForEncode)+1) // type, length, lvs_method are 1 byte each
	r[0] = IPOptionForExperiment            // type
	r[1] = byte(len(r))                     // length of option
	copy(r[2:], ipForEncode)
	r[len(r)-1] = lvsMethod
	return r
}

// For ipv6
func ConstructDstHeaderWithIP(ipForEncode net.IP, lvsMethod byte) []byte {
	payloadLen := byte(2 + 2 + len(ipForEncode) + 1)
	pad := (8 - payloadLen%8) % 8
	b := make([]byte, 0, payloadLen+pad)
	w := bytes.NewBuffer(b)
	// write DST ipv6 ext RFC 8200 4.6
	w.WriteByte(0)                  // nextHeader will be filled by system
	w.WriteByte(byte(cap(b)/8 - 1)) // extensionLen in octets excluding first
	// write DST option in TLV format. RFC 8200 4.2
	w.WriteByte(IPOptionForExperiment)      // type
	w.WriteByte(byte(len(ipForEncode) + 1)) // length of value
	w.Write(ipForEncode)                    // value
	w.WriteByte(lvsMethod)                  // lvs_method
	// padding to keep 8-byte alignment
	switch pad {
	case 0:
		break
	case 1:
		w.WriteByte(0) // Pad1
	default:
		padN := make([]byte, pad)
		padN[0] = 1       // type
		padN[1] = pad - 2 // count of zeros
		w.Write(padN)
	}
	return w.Bytes()
}
