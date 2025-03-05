package weight

import (
	"strconv"
)

// Weight represents a weight value. All negative values are considered omitted.
type Weight int

// Omitted is a special value indicating that the weight is not specified or
// omitted.
const Omitted Weight = -1

// Recalculate modifies the weight based on the provided coefficient and returns
// the new weight.
//
// If the weight is greater than the old value, it increases the weight by
// a calculated step, otherwise, it decreases the weight by that step.
// The step is calculated as `old * coeff / 100`, with a minimum step of 1.
//
// The function handles corner cases as follows:
//   - If `weight` is less than 0, it returns `old`.
//   - If `old` is less than 0, it returns `weight`.
//   - If `coeff` is 0, it returns `weight` without modification.
func (weight Weight) Recalculate(old Weight, coeff uint) Weight {
	if weight < 0 && old < 0 {
		return Omitted
	}
	if weight < 0 {
		return old
	}
	if old < 0 {
		return weight
	}

	if coeff == 0 {
		return weight
	}

	oldValue := int(old)
	step := oldValue * int(coeff) / 100
	if step == 0 {
		step = 1
	}

	if weight > old {
		newWeight := Weight(oldValue + step)
		return min(newWeight, weight)
	}

	newWeight := Weight(oldValue - step)
	return max(newWeight, weight)
}

func (weight Weight) String() string {
	if weight < 0 {
		return ""
	}
	return strconv.Itoa(int(weight))
}

// Uint32 returns the weight as a uint32 value.
// If the weight is less than 0, it returns 0.
func (weight Weight) Uint32() uint32 {
	if weight < 0 {
		return 0
	}
	return uint32(weight)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// It parses a text representation of the weight value.
// If the text cannot be parsed or represents a negative value, it sets the
// weight to Omitted.
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
