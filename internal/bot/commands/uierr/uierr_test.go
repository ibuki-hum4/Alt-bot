package uierr

import (
	"errors"
	"strings"
	"testing"

	"alt-bot/internal/service"
)

func TestFormatNil(t *testing.T) {
	if msg, ok := Format(nil, "獲得"); ok || msg != "" {
		t.Errorf("Format(nil) = (%q, %v), want (\"\", false)", msg, ok)
	}
}

func TestFormatUnknownError(t *testing.T) {
	if _, ok := Format(errors.New("boom"), "獲得"); ok {
		t.Error("Format(unknown error) should return recognized=false")
	}
}

func TestFormatInsufficientYen(t *testing.T) {
	msg, ok := Format(&service.InsufficientYenError{Need: 100, Have: 30}, "獲得")
	if !ok {
		t.Fatal("expected recognized=true")
	}
	if !strings.Contains(msg, "100") || !strings.Contains(msg, "30") {
		t.Errorf("Format InsufficientYenError = %q, want it to contain need/have amounts", msg)
	}
}

func TestFormatInsufficientALT(t *testing.T) {
	msg, ok := Format(&service.InsufficientALTError{Need: 5, Have: 2}, "獲得")
	if !ok {
		t.Fatal("expected recognized=true")
	}
	if !strings.Contains(msg, "5") || !strings.Contains(msg, "2") {
		t.Errorf("Format InsufficientALTError = %q, want it to contain need/have amounts", msg)
	}
}

func TestFormatProfitCapVerb(t *testing.T) {
	cases := []struct {
		name   string
		window string
		verb   string
		want   string
	}{
		{"daily_earn", "daily", "獲得", "本日の獲得上限"},
		{"weekly_earn", "weekly", "獲得", "今週の獲得上限"},
		{"daily_receive", "daily", "受取", "本日の受取上限"},
		{"weekly_receive", "weekly", "受取", "今週の受取上限"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg, ok := Format(&service.ProfitCapError{Window: c.window, Cap: 1000, Earned: 900}, c.verb)
			if !ok {
				t.Fatal("expected recognized=true")
			}
			if !strings.HasPrefix(msg, c.want) {
				t.Errorf("Format ProfitCapError = %q, want prefix %q", msg, c.want)
			}
		})
	}
}

func TestFormatShopErrors(t *testing.T) {
	msg, ok := Format(&service.ShopItemNotFoundError{ItemID: "potion"}, "獲得")
	if !ok || !strings.Contains(msg, "potion") {
		t.Errorf("Format ShopItemNotFoundError = (%q, %v), want it to contain item id", msg, ok)
	}

	msg, ok = Format(&service.ShopQuantityError{Requested: 10, Max: 3}, "獲得")
	if !ok || !strings.Contains(msg, "10") || !strings.Contains(msg, "3") {
		t.Errorf("Format ShopQuantityError = (%q, %v), want it to contain requested/max", msg, ok)
	}
}

func TestFormatMarketAndCircuitErrors(t *testing.T) {
	if _, ok := Format(&service.MarketHaltedError{}, "獲得"); !ok {
		t.Error("expected MarketHaltedError to be recognized")
	}
	if _, ok := Format(&service.CircuitLimitError{MaxQty: 10}, "獲得"); !ok {
		t.Error("expected CircuitLimitError to be recognized")
	}
	if _, ok := Format(&service.DailyIssuanceCapError{Cap: 100, Issued: 90}, "獲得"); !ok {
		t.Error("expected DailyIssuanceCapError to be recognized")
	}
}
