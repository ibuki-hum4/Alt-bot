package service

import (
	"context"
	"fmt"
	"time"

	"alt-bot/ent"
	"alt-bot/ent/user"
)

func (s *EconomyService) MinesPlaceBet(ctx context.Context, discordID string, bet int64) (int64, error) {
	if bet <= 0 {
		return 0, fmt.Errorf("bet must be positive")
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
					return fmt.Errorf("failed to create user in mines bet: %w", err)
				}
			} else {
				return fmt.Errorf("failed to load user in mines bet: %w", err)
			}
		}

		if u.Balance < bet {
			return &InsufficientYenError{Need: bet, Have: u.Balance}
		}

		newBalance := u.Balance - bet
		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in mines bet: %w", err)
		}

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         "casino_mines_bet",
			YenDelta:     -bet,
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

		balance = newBalance
		return nil
	})
	if err != nil {
		return 0, err
	}

	s.prevHash = nextHash
	return balance, nil
}

func (s *EconomyService) MinesCashout(ctx context.Context, discordID string, payout int64) (int64, error) {
	if payout < 0 {
		return 0, fmt.Errorf("payout must be non-negative")
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
			return fmt.Errorf("failed to load user in mines cashout: %w", err)
		}

		newBalance := u.Balance + payout
		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(newBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in mines cashout: %w", err)
		}

		nextHash, err = s.appendSignedLog(ctx, tx, txLogInput{
			DiscordID:    discordID,
			Kind:         "casino_mines_cashout",
			YenDelta:     payout,
			ALTDelta:     0,
			XPDelta:      0,
			Amount:       payout,
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
