package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"alt-bot/ent"
)

const (
	RobMinAmount int64 = 10
	RobMaxAmount int64 = 50000

	robCooldown      = 24 * time.Hour
	robSuccessChance = 0.5
)

type RobResult struct {
	AttackerDiscordID string
	TargetDiscordID   string
	Blocked           bool
	Success           bool
	Amount            int64
	AttackerBalance   int64
	TargetBalance     int64
}

type RobCooldownError struct {
	Until time.Time
}

func (e *RobCooldownError) Error() string {
	return fmt.Sprintf("rob is on cooldown until %s", e.Until.UTC().Format(time.RFC3339))
}

type RobTargetBalanceError struct {
	Need int64
	Have int64
}

func (e *RobTargetBalanceError) Error() string {
	return fmt.Sprintf("rob target balance too low: need=%d have=%d", e.Need, e.Have)
}

// robAmountRange returns the inclusive [min, max] yen range a rob attempt can
// move, capped by the target's balance so a rob never tries to take more
// than the target has.
func robAmountRange(targetBalance int64) (int64, int64) {
	max := RobMaxAmount
	if targetBalance < max {
		max = targetBalance
	}
	if max < RobMinAmount {
		max = RobMinAmount
	}
	return RobMinAmount, max
}

func (s *EconomyService) RobUser(ctx context.Context, attackerDiscordID, targetDiscordID string) (RobResult, error) {
	if attackerDiscordID == targetDiscordID {
		return RobResult{}, fmt.Errorf("cannot rob yourself")
	}

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	var result RobResult
	err := ent.WithTx(ctx, s.client, func(tx *ent.Tx) error {
		ids := []string{attackerDiscordID, targetDiscordID}
		sort.Strings(ids)

		users := make(map[string]*ent.User, 2)
		for _, discordID := range ids {
			u, err := s.lockOrCreateUserForUpdateTx(ctx, tx, discordID, "rob")
			if err != nil {
				return err
			}
			users[discordID] = u
		}

		attacker := users[attackerDiscordID]
		target := users[targetDiscordID]

		if attacker.RobEndAt.After(now) {
			return &RobCooldownError{Until: attacker.RobEndAt}
		}
		if target.Balance < RobMinAmount {
			return &RobTargetBalanceError{Need: RobMinAmount, Have: target.Balance}
		}

		nextRobAt := now.Add(robCooldown)

		if target.SecurityCameraCount > 0 {
			if _, err := tx.User.UpdateOneID(attacker.ID).
				SetRobEndAt(nextRobAt).
				Save(ctx); err != nil {
				return fmt.Errorf("failed to update attacker cooldown in rob: %w", err)
			}
			if _, err := tx.User.UpdateOneID(target.ID).
				SetSecurityCameraCount(target.SecurityCameraCount - 1).
				Save(ctx); err != nil {
				return fmt.Errorf("failed to consume security camera in rob: %w", err)
			}

			if _, err := s.appendSignedLog(ctx, tx, txLogInput{
				DiscordID:    attackerDiscordID,
				Kind:         "rob_blocked",
				BalanceAfter: attacker.Balance,
				ALTAfter:     attacker.CryptoBalance,
			}); err != nil {
				return err
			}

			result = RobResult{
				AttackerDiscordID: attackerDiscordID,
				TargetDiscordID:   targetDiscordID,
				Blocked:           true,
				AttackerBalance:   attacker.Balance,
				TargetBalance:     target.Balance,
			}
			return nil
		}

		minAmount, maxAmount := robAmountRange(target.Balance)
		amount := minAmount
		if maxAmount > minAmount {
			amount = minAmount + s.rand.Int63n(maxAmount-minAmount+1)
		}

		success := s.rand.Float64() < robSuccessChance

		var attackerBalance, targetBalance int64
		kind := "rob_fail"
		if success {
			if err := s.recordProfitCapsTx(ctx, tx, attacker, amount, now); err != nil {
				return err
			}
			attackerBalance = attacker.Balance + amount
			targetBalance = target.Balance - amount
			kind = "rob_success"
		} else {
			fine := amount
			if fine > attacker.Balance {
				fine = attacker.Balance
			}
			amount = fine
			attackerBalance = attacker.Balance - fine
			targetBalance = target.Balance + fine
		}

		if _, err := tx.User.UpdateOneID(attacker.ID).
			SetBalance(attackerBalance).
			SetRobEndAt(nextRobAt).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update attacker in rob: %w", err)
		}
		if _, err := tx.User.UpdateOneID(target.ID).
			SetBalance(targetBalance).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to update target in rob: %w", err)
		}

		if _, err := s.appendSignedLogChain(ctx, tx,
			txLogInput{
				DiscordID:    attackerDiscordID,
				Kind:         kind,
				YenDelta:     attackerBalance - attacker.Balance,
				BalanceAfter: attackerBalance,
				ALTAfter:     attacker.CryptoBalance,
			},
			txLogInput{
				DiscordID:    targetDiscordID,
				Kind:         kind,
				YenDelta:     targetBalance - target.Balance,
				BalanceAfter: targetBalance,
				ALTAfter:     target.CryptoBalance,
			},
		); err != nil {
			return err
		}

		result = RobResult{
			AttackerDiscordID: attackerDiscordID,
			TargetDiscordID:   targetDiscordID,
			Blocked:           false,
			Success:           success,
			Amount:            amount,
			AttackerBalance:   attackerBalance,
			TargetBalance:     targetBalance,
		}
		return nil
	})
	if err != nil {
		return RobResult{}, err
	}

	return result, nil
}
