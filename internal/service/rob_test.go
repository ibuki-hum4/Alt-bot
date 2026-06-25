package service

import "testing"

func TestRobAmountRange(t *testing.T) {
	cases := []struct {
		name          string
		targetBalance int64
		wantMin       int64
		wantMax       int64
	}{
		{"below_minimum_clamped_to_minimum", 5, RobMinAmount, RobMinAmount},
		{"exactly_minimum", RobMinAmount, RobMinAmount, RobMinAmount},
		{"between_min_and_max", 1_000, RobMinAmount, 1_000},
		{"exactly_maximum", RobMaxAmount, RobMinAmount, RobMaxAmount},
		{"above_maximum_clamped_to_maximum", 1_000_000, RobMinAmount, RobMaxAmount},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			min, max := robAmountRange(c.targetBalance)
			if min != c.wantMin || max != c.wantMax {
				t.Errorf("robAmountRange(%d) = (%d, %d), want (%d, %d)", c.targetBalance, min, max, c.wantMin, c.wantMax)
			}
			if min > max {
				t.Errorf("robAmountRange(%d) returned min(%d) > max(%d)", c.targetBalance, min, max)
			}
		})
	}
}
