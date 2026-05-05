package service

import (
	"context"
	"fmt"
	"time"

	"alt-bot/ent"
	"alt-bot/ent/user"
)

func (s *EconomyService) BlackjackPlaceBet(ctx context.Context, discordID string, bet int64) (int64, error) {
	return s.blackjackAdjustBalance(ctx, discordID, bet, -bet, "casino_blackjack_bet")
}

func (s *EconomyService) BlackjackAddBet(ctx context.Context, discordID string, bet int64) (int64, error) {
	return s.blackjackAdjustBalance(ctx, discordID, bet, -bet, "casino_blackjack_add_bet")
}

func (s *EconomyService) BlackjackCashout(ctx context.Context, discordID string, payout int64) (int64, error) {
	if payout < 0 {
		return 0, fmt.Errorf("payout must be non-negative")
	}
	return s.blackjackAdjustBalance(ctx, discordID, payout, payout, "casino_blackjack_cashout")
}

func (s *EconomyService) blackjackAdjustBalance(ctx context.Context, discordID string, amount int64, delta int64, kind string) (int64, error) {
	if amount < 0 {
		return 0, fmt.Errorf("amount must be non-negative")
	}
	if amount == 0 && delta != 0 {
		return 0, fmt.Errorf("amount must be positive for non-zero delta")
	}

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	var balance int64
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
					return fmt.Errorf("failed to create user in blackjack: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load user in blackjack: %w", err)
			}
		}

		if delta < 0 && u.Balance < -delta {
			return &InsufficientYenError{Need: -delta, Have: u.Balance}
		}

		newBalance := u.Balance + delta
		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in blackjack: %w", err)
		}

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         kind,
			YenDelta:     delta,
			ALTDelta:     0,
			XPDelta:      0,
			Amount:       amount,
			SettledPrice: 0,
			PriceAfter:   0,
			BalanceAfter: newBalance,
			ALTAfter:     u.CryptoBalance,
		})
		if err != nil {
			return err
		}

		balance = newBalance
		return nil
	})
	if err != nil {
		return 0, err
	}

	s.prevHash = nextHash
	return balance, nil
}
