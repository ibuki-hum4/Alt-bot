package service

import (
	"testing"

	"alt-bot/internal/config"
)

func newTestEconomyService(cfg config.Config) *EconomyService {
	return &EconomyService{cfg: cfg}
}

func TestCalculateCasinoFee(t *testing.T) {
	s := newTestEconomyService(config.Config{CasinoFeePercent: 0.01})

	fee, after := s.CalculateCasinoFee(1000)
	if fee != 10 || after != 990 {
		t.Errorf("CalculateCasinoFee(1000) = (%d, %d), want (10, 990)", fee, after)
	}

	// Minimum fee of 1 yen even when the percentage rounds down to 0.
	fee, after = s.CalculateCasinoFee(10)
	if fee != 1 || after != 9 {
		t.Errorf("CalculateCasinoFee(10) = (%d, %d), want (1, 9)", fee, after)
	}

	if fee, after := s.CalculateCasinoFee(0); fee != 0 || after != 0 {
		t.Errorf("CalculateCasinoFee(0) = (%d, %d), want (0, 0)", fee, after)
	}
}

func TestCalculateCasinoFeeDisabled(t *testing.T) {
	s := newTestEconomyService(config.Config{CasinoFeePercent: 0})
	fee, after := s.CalculateCasinoFee(1000)
	if fee != 0 || after != 1000 {
		t.Errorf("CalculateCasinoFee with 0%% fee = (%d, %d), want (0, 1000)", fee, after)
	}
}

func TestCalculateCashoutFee(t *testing.T) {
	s := newTestEconomyService(config.Config{CashoutFeePercent: 0.05})
	fee, after := s.CalculateCashoutFee(1000)
	if fee != 50 || after != 950 {
		t.Errorf("CalculateCashoutFee(1000) = (%d, %d), want (50, 950)", fee, after)
	}
}

func TestCalculateHighValueTax(t *testing.T) {
	s := newTestEconomyService(config.Config{})

	cases := []struct {
		amount    int64
		wantTax   int64
		wantAfter int64
	}{
		{amount: 5_000, wantTax: 0, wantAfter: 5_000},        // below first tier
		{amount: 10_000, wantTax: 1_500, wantAfter: 8_500},   // 15%
		{amount: 100_000, wantTax: 20_000, wantAfter: 80_000}, // 20%
		{amount: 1_000_000, wantTax: 250_000, wantAfter: 750_000}, // 25%
		{amount: 0, wantTax: 0, wantAfter: 0},
	}
	for _, c := range cases {
		tax, after := s.CalculateHighValueTax(c.amount)
		if tax != c.wantTax || after != c.wantAfter {
			t.Errorf("CalculateHighValueTax(%d) = (%d, %d), want (%d, %d)", c.amount, tax, after, c.wantTax, c.wantAfter)
		}
	}
}

func TestMaxBetForUser(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.Config
		balance int64
		want    int64
	}{
		{"no_limits", config.Config{}, 10_000, 10_000},
		{"amount_limit_only", config.Config{MaxBetAmount: 500}, 10_000, 500},
		{"percent_limit_only", config.Config{MaxBetPercent: 0.1}, 10_000, 1_000},
		{"min_of_both", config.Config{MaxBetAmount: 5_000, MaxBetPercent: 0.1}, 10_000, 1_000},
		{"amount_lower_than_percent", config.Config{MaxBetAmount: 200, MaxBetPercent: 0.5}, 10_000, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestEconomyService(c.cfg)
			if got := s.MaxBetForUser(c.balance); got != c.want {
				t.Errorf("MaxBetForUser(%d) = %d, want %d", c.balance, got, c.want)
			}
		})
	}
}
