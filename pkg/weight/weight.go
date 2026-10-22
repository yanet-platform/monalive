package weight

import (
	"strconv"
)

// Weight represents a weight value. All negative values are considered omitted.
type Weight int

// Omitted is a special value indicating that the weight is not specified or
// omitted.
const Omitted Weight = -1

func (weight Weight) String() string {
	if weight < 0 {
		return ""
	}
	return strconv.Itoa(int(weight))
}

// Uint32 returns the weight as a uint32 value. If the weight is less than 0, it
// returns 0.
func (weight Weight) Uint32() uint32 {
	if weight < 0 {
		return 0
	}
	return uint32(weight)
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface. It parses a
// text representation of the weight value. If the text cannot be parsed or
// represents a negative value, it sets the weight to [Omitted].
func (weight *Weight) UnmarshalText(text []byte) error {
	s := string(text)
	w, err := strconv.Atoi(s)
	if err != nil {
		*weight = Omitted
		return nil
	}

	if w < 0 {
		*weight = Omitted
		return nil
	}

	*weight = Weight(w)
	return nil
}
