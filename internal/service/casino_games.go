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
	CasinoMinBet int64 = 1
	CasinoMaxBet int64 = 100000
)

type CasinoPlayResult struct {
	BetYen     int64
	PayoutYen  int64
	NetYen     int64
	Multiplier float64
	Symbols    []string
	YenBalance int64
	AltBalance int64
}

type weightedCasinoOutcome struct {
	weight     int
	multiplier float64
	symbols    []string
}

func (s *EconomyService) PlayBlackjack(ctx context.Context, discordID string, bet int64) (CasinoPlayResult, error) {
	outcomes := []weightedCasinoOutcome{
		{weight: 70, multiplier: 2.0, symbols: []string{"BLACKJACK", "A+10"}},
		{weight: 350, multiplier: 1.8, symbols: []string{"WIN", "20 vs 18"}},
		{weight: 240, multiplier: 1.0, symbols: []string{"PUSH", "19 vs 19"}},
		{weight: 340, multiplier: 0.0, symbols: []string{"LOSE", "BUST"}},
	}
	return s.playWeightedCasino(ctx, discordID, bet, "casino_blackjack", outcomes, s.casinoRTPBlackjack)
}

func (s *EconomyService) PlayChinchiro(ctx context.Context, discordID string, bet int64) (CasinoPlayResult, error) {
	outcomes := []weightedCasinoOutcome{
		{weight: 10, multiplier: 4.0, symbols: []string{"123", "Hifumi Win"}},
		{weight: 90, multiplier: 2.0, symbols: []string{"Shigoro", "Strong"}},
		{weight: 280, multiplier: 1.6, symbols: []string{"Point Win", "Normal"}},
		{weight: 140, multiplier: 1.0, symbols: []string{"Draw", "Re-roll"}},
		{weight: 480, multiplier: 0.0, symbols: []string{"Point Lose", "Normal"}},
	}
	return s.playWeightedCasino(ctx, discordID, bet, "casino_chinchiro", outcomes, s.casinoRTPChinchiro)
}

func (s *EconomyService) PlayMines(ctx context.Context, discordID string, bet int64) (CasinoPlayResult, error) {
	outcomes := []weightedCasinoOutcome{
		{weight: 520, multiplier: 0.0, symbols: []string{"BOOM", "Mine"}},
		{weight: 220, multiplier: 1.2, symbols: []string{"SAFE", "x1"}},
		{weight: 150, multiplier: 1.6, symbols: []string{"SAFE", "x2"}},
		{weight: 80, multiplier: 2.4, symbols: []string{"SAFE", "x3"}},
		{weight: 25, multiplier: 3.6, symbols: []string{"SAFE", "x4"}},
		{weight: 5, multiplier: 6.0, symbols: []string{"JACKPOT", "x5"}},
	}
	return s.playWeightedCasino(ctx, discordID, bet, "casino_mines", outcomes, s.casinoRTPMines)
}

func (s *EconomyService) PlayPoker(ctx context.Context, discordID string, bet int64) (CasinoPlayResult, error) {
	outcomes := []weightedCasinoOutcome{
		{weight: 5, multiplier: 25.0, symbols: []string{"ROYAL", "A K Q J 10"}},
		{weight: 20, multiplier: 8.0, symbols: []string{"FULLHOUSE", "Trips + Pair"}},
		{weight: 120, multiplier: 3.0, symbols: []string{"TWO-PAIR", "Strong"}},
		{weight: 220, multiplier: 1.5, symbols: []string{"PAIR", "Small win"}},
		{weight: 635, multiplier: 0.0, symbols: []string{"HIGH-CARD", "Lose"}},
	}
	return s.playWeightedCasino(ctx, discordID, bet, "casino_poker", outcomes, s.casinoRTPPoker)
}

func (s *EconomyService) playWeightedCasino(ctx context.Context, discordID string, bet int64, kind string, outcomes []weightedCasinoOutcome, targetRTP float64) (CasinoPlayResult, error) {
	if bet < CasinoMinBet || bet > CasinoMaxBet {
		return CasinoPlayResult{}, fmt.Errorf("casino bet must be between %d and %d", CasinoMinBet, CasinoMaxBet)
	}

	totalWeight := 0
	for _, out := range outcomes {
		totalWeight += out.weight
	}
	if totalWeight <= 0 {
		return CasinoPlayResult{}, fmt.Errorf("invalid casino outcomes")
	}

	entryFee, totalCost := s.CalculateCasinoFee(bet)

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	draw := s.rand.Intn(totalWeight)
	picked := drawCasinoOutcome(draw, outcomes)
	scaledMultiplier := picked.multiplier * rtpScaleForTarget(targetRTP, weightedBaseRTP(outcomes))
	grossPayout := int64(math.Floor(float64(bet) * scaledMultiplier))
	_, payout := s.CalculateHighValueTax(grossPayout)
	net := payout - bet - entryFee

	var result CasinoPlayResult
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
					return fmt.Errorf("failed to create user in casino: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load user in casino: %w", err)
			}
		}

		if u.Balance < totalCost {
			return &InsufficientYenError{Need: totalCost, Have: u.Balance}
		}

		newBalance := u.Balance - totalCost + payout
		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in casino: %w", err)
		}

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         kind,
			YenDelta:     net,
			ALTDelta:     0,
			XPDelta:      0,
			Amount:       totalCost,
			SettledPrice: 0,
			PriceAfter:   0,
			BalanceAfter: newBalance,
			ALTAfter:     u.CryptoBalance,
		})
		if err != nil {
			return err
		}

		result = CasinoPlayResult{
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
		return CasinoPlayResult{}, err
	}

	s.prevHash = nextHash
	return result, nil
}

func drawCasinoOutcome(draw int, outcomes []weightedCasinoOutcome) weightedCasinoOutcome {
	acc := 0
	for _, out := range outcomes {
		acc += out.weight
		if draw < acc {
			return out
		}
	}
	return outcomes[len(outcomes)-1]
}

func weightedBaseRTP(outcomes []weightedCasinoOutcome) float64 {
	totalWeight := 0
	expected := 0.0
	for _, out := range outcomes {
		totalWeight += out.weight
		expected += float64(out.weight) * out.multiplier
	}
	if totalWeight <= 0 {
		return 1.0
	}
	return expected / float64(totalWeight)
}

func rtpScaleForTarget(targetRTP float64, baseRTP float64) float64 {
	if baseRTP <= 0 {
		return 1.0
	}
	scale := targetRTP / baseRTP
	if scale < 0.1 {
		return 0.1
	}
	if scale > 3.0 {
		return 3.0
	}
	return scale
}
