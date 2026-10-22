package checker

import (
	"testing"

	"github.com/yanet-platform/monalive/pkg/weight"
)

func TestRecalculate(t *testing.T) {
	tests := []struct {
		TestID    uint
		NewWeight weight.Weight
		OldWeight weight.Weight
		Coeff     uint
		Want      weight.Weight
	}{
		// NewWeight > OldWeight.
		{1, weight.Weight(20), weight.Weight(10), 0, weight.Weight(20)},
		{2, weight.Weight(20), weight.Weight(10), 1, weight.Weight(11)},
		{3, weight.Weight(20), weight.Weight(10), 40, weight.Weight(14)},
		{4, weight.Weight(20), weight.Weight(10), 100, weight.Weight(20)},
		{5, weight.Weight(20), weight.Weight(10), 146, weight.Weight(20)},

		// NewWeight < OldWeight.
		{6, weight.Weight(10), weight.Weight(20), 0, weight.Weight(10)},
		{7, weight.Weight(10), weight.Weight(20), 1, weight.Weight(19)},
		{8, weight.Weight(10), weight.Weight(20), 40, weight.Weight(12)},
		{9, weight.Weight(10), weight.Weight(20), 100, weight.Weight(10)},
		{10, weight.Weight(10), weight.Weight(20), 146, weight.Weight(10)},

		// NewWeight < 0, OldWeight >= 0.
		{11, weight.Weight(-1), weight.Weight(20), 0, weight.Weight(20)},
		{12, weight.Weight(-1), weight.Weight(20), 1, weight.Weight(20)},
		{13, weight.Weight(-1), weight.Weight(20), 40, weight.Weight(20)},
		{14, weight.Weight(-1), weight.Weight(20), 100, weight.Weight(20)},
		{15, weight.Weight(-1), weight.Weight(20), 146, weight.Weight(20)},

		// OldWeight < 0, NewWeight >= 0.
		{16, weight.Weight(10), weight.Weight(-1), 0, weight.Weight(10)},
		{17, weight.Weight(10), weight.Weight(-1), 1, weight.Weight(10)},
		{18, weight.Weight(10), weight.Weight(-1), 40, weight.Weight(10)},
		{19, weight.Weight(10), weight.Weight(-1), 100, weight.Weight(10)},
		{20, weight.Weight(10), weight.Weight(-1), 146, weight.Weight(10)},

		// NewWeight = OldWeight, both >= 0.
		{21, weight.Weight(10), weight.Weight(10), 0, weight.Weight(10)},
		{22, weight.Weight(10), weight.Weight(10), 1, weight.Weight(10)},
		{23, weight.Weight(10), weight.Weight(10), 40, weight.Weight(10)},
		{24, weight.Weight(10), weight.Weight(10), 100, weight.Weight(10)},
		{25, weight.Weight(10), weight.Weight(10), 146, weight.Weight(10)},

		// OldWeight < 0, NewWeight < 0.
		{26, weight.Weight(-1), weight.Weight(-3), 0, weight.Omitted},
		{27, weight.Weight(-1), weight.Weight(-3), 1, weight.Omitted},
		{28, weight.Weight(-1), weight.Weight(-3), 40, weight.Omitted},
		{29, weight.Weight(-1), weight.Weight(-3), 100, weight.Omitted},
		{30, weight.Weight(-1), weight.Weight(-3), 146, weight.Omitted},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			checker := &Checker{config: &Config{}}
			checker.state.Weight = tt.OldWeight
			checker.config.DynamicWeightCoeff = tt.Coeff
			got := checker.recalculateWeight(tt.NewWeight)
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
