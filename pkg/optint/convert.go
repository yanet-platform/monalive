package optint

import (
	"strconv"

	"golang.org/x/exp/constraints"
)

// ToString converts an optional integer to a string. Returns an empty string if
// the num is nil.
func ToString[T constraints.Integer](num *T) string {
	if num == nil {
		return ""
	}
	return strconv.Itoa(int(*num))
}
