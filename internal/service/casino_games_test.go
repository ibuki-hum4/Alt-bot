package service

import "testing"

func TestDrawCasinoOutcome(t *testing.T) {
	outcomes := []weightedCasinoOutcome{
		{weight: 10, multiplier: 2.0},
		{weight: 20, multiplier: 1.0},
		{weight: 70, multiplier: 0.0},
	}

	cases := []struct {
		draw int
		want float64
	}{
		{0, 2.0},
		{9, 2.0},
		{10, 1.0},
		{29, 1.0},
		{30, 0.0},
		{99, 0.0},
	}
	for _, c := range cases {
		got := drawCasinoOutcome(c.draw, outcomes)
		if got.multiplier != c.want {
			t.Errorf("drawCasinoOutcome(%d) multiplier = %v, want %v", c.draw, got.multiplier, c.want)
		}
	}
}

func TestDrawCasinoOutcomeOutOfRangeFallsBackToLast(t *testing.T) {
	outcomes := []weightedCasinoOutcome{
		{weight: 10, multiplier: 2.0},
		{weight: 10, multiplier: 0.0},
	}
	got := drawCasinoOutcome(1000, outcomes)
	if got.multiplier != 0.0 {
		t.Errorf("drawCasinoOutcome out of range = %v, want last outcome multiplier 0.0", got.multiplier)
	}
}

func TestWeightedBaseRTP(t *testing.T) {
	outcomes := []weightedCasinoOutcome{
		{weight: 50, multiplier: 2.0},
		{weight: 50, multiplier: 0.0},
	}
	// expected = (50*2.0 + 50*0.0) / 100 = 1.0
	if got := weightedBaseRTP(outcomes); got != 1.0 {
		t.Errorf("weightedBaseRTP = %v, want 1.0", got)
	}
}

func TestWeightedBaseRTPZeroWeight(t *testing.T) {
	if got := weightedBaseRTP(nil); got != 1.0 {
		t.Errorf("weightedBaseRTP(nil) = %v, want 1.0", got)
	}
}

func TestRtpScaleForTarget(t *testing.T) {
	cases := []struct {
		name      string
		targetRTP float64
		baseRTP   float64
		want      float64
	}{
		{"equal", 1.0, 1.0, 1.0},
		{"half_target", 0.5, 1.0, 0.5},
		{"zero_base_returns_one", 0.9, 0, 1.0},
		{"clamped_low", 0.01, 1.0, 0.1},
		{"clamped_high", 10.0, 1.0, 3.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rtpScaleForTarget(c.targetRTP, c.baseRTP); got != c.want {
				t.Errorf("rtpScaleForTarget(%v, %v) = %v, want %v", c.targetRTP, c.baseRTP, got, c.want)
			}
		})
	}
}
