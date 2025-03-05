package weight

import (
	"testing"
)

func TestRecalculate(t *testing.T) {
	tests := []struct {
		TestID    uint
		NewWeight Weight
		OldWeight Weight
		Coeff     uint
		Want      Weight
	}{
		// NewWeight > OldWeight.
		{1, Weight(20), Weight(10), 0, Weight(20)},
		{2, Weight(20), Weight(10), 1, Weight(11)},
		{3, Weight(20), Weight(10), 40, Weight(14)},
		{4, Weight(20), Weight(10), 100, Weight(20)},
		{5, Weight(20), Weight(10), 146, Weight(20)},

		// NewWeight < OldWeight.
		{6, Weight(10), Weight(20), 0, Weight(10)},
		{7, Weight(10), Weight(20), 1, Weight(19)},
		{8, Weight(10), Weight(20), 40, Weight(12)},
		{9, Weight(10), Weight(20), 100, Weight(10)},
		{10, Weight(10), Weight(20), 146, Weight(10)},

		// NewWeight < 0, OldWeight >= 0.
		{11, Weight(-1), Weight(20), 0, Weight(20)},
		{12, Weight(-1), Weight(20), 1, Weight(20)},
		{13, Weight(-1), Weight(20), 40, Weight(20)},
		{14, Weight(-1), Weight(20), 100, Weight(20)},
		{15, Weight(-1), Weight(20), 146, Weight(20)},

		// OldWeight < 0, NewWeight >= 0.
		{16, Weight(10), Weight(-1), 0, Weight(10)},
		{17, Weight(10), Weight(-1), 1, Weight(10)},
		{18, Weight(10), Weight(-1), 40, Weight(10)},
		{19, Weight(10), Weight(-1), 100, Weight(10)},
		{20, Weight(10), Weight(-1), 146, Weight(10)},

		// NewWeight = OldWeight, both >= 0.
		{21, Weight(10), Weight(10), 0, Weight(10)},
		{22, Weight(10), Weight(10), 1, Weight(10)},
		{23, Weight(10), Weight(10), 40, Weight(10)},
		{24, Weight(10), Weight(10), 100, Weight(10)},
		{25, Weight(10), Weight(10), 146, Weight(10)},

		// OldWeight < 0, NewWeight < 0.
		{26, Weight(-1), Weight(-3), 0, Omitted},
		{27, Weight(-1), Weight(-3), 1, Omitted},
		{28, Weight(-1), Weight(-3), 40, Omitted},
		{29, Weight(-1), Weight(-3), 100, Omitted},
		{30, Weight(-1), Weight(-3), 146, Omitted},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := tt.NewWeight.Recalculate(tt.OldWeight, tt.Coeff)
			if got != tt.Want {
				t.Errorf(
					"TestID %d: Recalculate(%v, %v, %v) = %v; want %v",
					tt.TestID,
					tt.NewWeight,
					tt.OldWeight,
					tt.Coeff,
					got,
					tt.Want,
				)
			}
		})
	}
}
