package service

import (
	"context"
	"fmt"
	"time"

	"alt-bot/ent"
	"alt-bot/ent/user"
)

func (s *EconomyService) lockUserForUpdateTx(ctx context.Context, tx *ent.Tx, discordID string) (*ent.User, error) {
	u, err := tx.User.Query().
		Where(user.DiscordID(discordID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *EconomyService) lockOrCreateUserForUpdateTx(ctx context.Context, tx *ent.Tx, discordID string, reason string) (*ent.User, error) {
	u, err := s.lockUserForUpdateTx(ctx, tx, discordID)
	if err == nil {
		return u, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to load user in %s: %w", reason, err)
	}

	u, err = tx.User.Create().
		SetDiscordID(discordID).
		SetBalance(0).
		SetCryptoBalance(0).
		SetXp(0).
		SetWorkEndAt(time.Unix(0, 0).UTC()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user in %s: %w", reason, err)
	}
	return u, nil
}