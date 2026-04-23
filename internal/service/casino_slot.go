package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"alt-bot/ent"
	"alt-bot/ent/user"
)

const (
	SlotMinBet       int64 = 1
	SlotMaxBet       int64 = 100000
	slotWeightTotal        = 1000
	slotBaseRTP            = 0.959
)

type SlotPlayResult struct {
	BetYen     int64
	PayoutYen  int64
	NetYen     int64
	Multiplier float64
	Symbols    []string
	YenBalance int64
	AltBalance int64
}

type slotOutcome struct {
	weight     int
	multiplier float64
	symbols    []string
}

var slotOutcomes = []slotOutcome{
	// Target RTP ~= 95.9% (long-run expected payout / bet).
	{weight: 4, multiplier: 20.0, symbols: []string{"7", "7", "7"}},
	{weight: 12, multiplier: 8.0, symbols: []string{"BAR", "BAR", "BAR"}},
	{weight: 24, multiplier: 4.0, symbols: []string{"BELL", "BELL", "BELL"}},
	{weight: 90, multiplier: 2.0, symbols: []string{"CHERRY", "CHERRY", "LEMON"}},
	{weight: 280, multiplier: 1.5, symbols: []string{"CHERRY", "LEMON", "BAR"}},
	{weight: 290, multiplier: 0.3, symbols: []string{"7", "7", "BAR"}},
	{weight: 300, multiplier: 0.0, symbols: []string{"LEMON", "BAR", "BELL"}},
}

func (s *EconomyService) PlaySlot(ctx context.Context, discordID string, bet int64) (SlotPlayResult, error) {
	if bet < SlotMinBet || bet > SlotMaxBet {
		return SlotPlayResult{}, fmt.Errorf("slot bet must be between %d and %d", SlotMinBet, SlotMaxBet)
	}

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	picked := drawSlotOutcome(s.rand.Intn(slotWeightTotal))
	scaledMultiplier := picked.multiplier * rtpScaleForTarget(s.casinoRTPSlot, slotBaseRTP)
	payout := int64(math.Floor(float64(bet) * scaledMultiplier))
	net := payout - bet

	var result SlotPlayResult
	var nextHash string
	err := ent.WithTx(ctx, s.client, func(tx *ent.Tx) error {
		u, err := tx.User.Query().
			Where(user.DiscordID(discordID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				u, err = tx.User.Create().
					SetDiscordID(discordID).
					SetBalance(0).
					SetCryptoBalance(0).
					SetXp(0).
					SetWorkEndAt(time.Unix(0, 0).UTC()).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to create user in slot: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load user in slot: %w", err)
			}
		}

		if u.Balance < bet {
			return &InsufficientYenError{Need: bet, Have: u.Balance}
		}

		newBalance := u.Balance + net
		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in slot: %w", err)
		}

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         "casino_slot",
			YenDelta:     net,
			ALTDelta:     0,
			XPDelta:      0,
			Amount:       bet,
			SettledPrice: 0,
			PriceAfter:   0,
			BalanceAfter: newBalance,
			ALTAfter:     u.CryptoBalance,
		})
		if err != nil {
			return err
		}

		result = SlotPlayResult{
			BetYen:     bet,
			PayoutYen:  payout,
			NetYen:     net,
			Multiplier: scaledMultiplier,
			Symbols:    append([]string(nil), picked.symbols...),
			YenBalance: newBalance,
			AltBalance: u.CryptoBalance,
		}
		return nil
	})
	if err != nil {
		return SlotPlayResult{}, err
	}

	s.prevHash = nextHash
	return result, nil
}

func drawSlotOutcome(draw int) slotOutcome {
	acc := 0
	for _, out := range slotOutcomes {
		acc += out.weight
		if draw < acc {
			return out
		}
	}
	return slotOutcomes[len(slotOutcomes)-1]
}
