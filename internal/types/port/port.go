package port

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Port represents a physical port number.
//
// Port is an integer type used to represent physical ports. Since port values
// can be optional (omitted for L3 checks) and are part of map-key types, it is
// implemented as an int32 rather than a uint16. All negative values are
// considered omitted.
type Port int32

// Omitted is a special value indicating that the port is not specified or
// omitted.
const Omitted Port = -1

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// It parses the text representation of the port and sets the Port value.
// An empty string is interpreted as an omitted port.
func (p *Port) UnmarshalText(text []byte) error {
	port := Omitted
	if str := string(text); str != "" {
		// Parsing the port value as uint16 because, if set, it must conform to
		// this type.
		value, err := strconv.ParseUint(str, 10, 16)
		if err != nil {
			return fmt.Errorf("invalid port value %q: %w", str, err)
		}
		port = Port(value)
	}

	*p = port

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
// It serializes the Port value as a JSON number. If the Port value is omitted,
// it serializes as null.
func (p Port) MarshalJSON() ([]byte, error) {
	if p < 0 {
		return json.Marshal(nil)
	}
	return json.Marshal(int32(p))
}

// Returns the port value as uint16. If the port is [Omitted], returns 0, which
// signals the system to automatically assign a port when creating a socket.
func (p Port) Value() uint16 {
	if p < 0 {
		return 0
	}
	return uint16(p)
}

// String returns the string representation of the Port value.
// If the Port value is omitted, it returns an empty string.
func (p Port) String() string {
	if p < 0 {
		return ""
	}
	return strconv.Itoa(int(p))
}

// ProtoMarshaller converts the Port value to a pointer to a uint32 for use in
// protocol buffer messages. If the Port value is omitted, it returns nil.
func (p Port) ProtoMarshaller() *uint32 {
	if p < 0 {
		return nil
	}
	v := uint32(p)
	return &v
}
