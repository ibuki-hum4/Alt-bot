package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"alt-bot/ent"
)

type PayResult struct {
	Amount       int64
	FromBalance  int64
	ToBalance    int64
	FromDiscordID string
	ToDiscordID   string
}

func (s *EconomyService) PayYen(ctx context.Context, fromDiscordID, toDiscordID string, amount int64) (PayResult, error) {
	if amount <= 0 {
		return PayResult{}, fmt.Errorf("amount must be positive")
	}
	if fromDiscordID == toDiscordID {
		return PayResult{}, fmt.Errorf("cannot pay yourself")
	}

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	var result PayResult
	var nextHash string
	err := ent.WithTx(ctx, s.client, func(tx *ent.Tx) error {
		ids := []string{fromDiscordID, toDiscordID}
		sort.Strings(ids)

		users := make(map[string]*ent.User, 2)
		for _, discordID := range ids {
			u, err := s.lockOrCreateUserForUpdateTx(ctx, tx, discordID, "pay")
			if err != nil {
				return err
			}
			users[discordID] = u
		}

		sender := users[fromDiscordID]
		receiver := users[toDiscordID]
		if sender.Balance < amount {
			return &InsufficientYenError{Need: amount, Have: sender.Balance}
		}

		now := time.Now().UTC()
		if err := s.recordProfitCapsTx(ctx, tx, receiver, amount, now); err != nil {
			return err
		}

		senderBalance := sender.Balance - amount
		receiverBalance := receiver.Balance + amount

		if _, err := tx.User.UpdateOneID(sender.ID).
			SetBalance(senderBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update sender balance in pay: %w", err)
		}
		if _, err := tx.User.UpdateOneID(receiver.ID).
			SetBalance(receiverBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update receiver balance in pay: %w", err)
		}

		prevHash := s.prevHash
		senderHash, err := s.appendSignedLogWithPrevHash(ctx, tx, prevHash, txLogInput{
			DiscordID:    fromDiscordID,
			Kind:         "pay_out",
			YenDelta:     -amount,
			ALTDelta:     0,
			XPDelta:      0,
			Amount:       amount,
			SettledPrice:  0,
			PriceAfter:    0,
			BalanceAfter:  senderBalance,
			ALTAfter:      sender.CryptoBalance,
		})
		if err != nil {
			return err
		}
		receiverHash, err := s.appendSignedLogWithPrevHash(ctx, tx, senderHash, txLogInput{
			DiscordID:    toDiscordID,
			Kind:         "pay_in",
			YenDelta:     amount,
			ALTDelta:     0,
			XPDelta:      0,
			Amount:       amount,
			SettledPrice:  0,
			PriceAfter:    0,
			BalanceAfter:  receiverBalance,
			ALTAfter:      receiver.CryptoBalance,
		})
		if err != nil {
			return err
		}

		nextHash = receiverHash
		result = PayResult{
			Amount:        amount,
			FromBalance:   senderBalance,
			ToBalance:     receiverBalance,
			FromDiscordID:  fromDiscordID,
			ToDiscordID:    toDiscordID,
		}
		return nil
	})
	if err != nil {
		return PayResult{}, err
	}

	s.prevHash = nextHash
	return result, nil
}
