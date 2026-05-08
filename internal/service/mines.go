package service

import (
	"context"
	"fmt"

	"alt-bot/ent"
	"alt-bot/ent/user"
)

func (s *EconomyService) MinesPlaceBet(ctx context.Context, discordID string, bet int64) (int64, error) {
	if bet <= 0 {
		return 0, fmt.Errorf("bet must be positive")
	}

	_, totalDebit := s.CalculateCasinoFee(bet)
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	var balance int64
	err := ent.WithTx(ctx, s.client, func(tx *ent.Tx) error {
		u, err := tx.User.Query().
			Where(user.DiscordID(discordID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return &InsufficientYenError{Need: totalDebit, Have: 0}
			}
			return fmt.Errorf("failed to load user in mines bet: %w", err)
		}

		if u.Balance < totalDebit {
			return &InsufficientYenError{Need: totalDebit, Have: u.Balance}
		}

		balance = u.Balance - totalDebit
		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(balance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in mines bet: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return balance, nil
}

func (s *EconomyService) MinesCashout(ctx context.Context, discordID string, payout int64) (int64, error) {
	if payout < 0 {
		return 0, fmt.Errorf("payout must be non-negative")
	}

	_, netPayout := s.CalculateHighValueTax(payout)
	if netPayout <= 0 {
		return 0, nil
	}

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	var balance int64
	err := ent.WithTx(ctx, s.client, func(tx *ent.Tx) error {
		u, err := tx.User.Query().
			Where(user.DiscordID(discordID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			return fmt.Errorf("failed to load user in mines cashout: %w", err)
		}

		balance = u.Balance + netPayout
		if _, err = tx.User.UpdateOneID(u.ID).
			SetBalance(balance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update user in mines cashout: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return balance, nil
}
