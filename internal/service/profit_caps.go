package service

import (
	"context"
	"fmt"	
	"time"

	"alt-bot/ent"
)

type ProfitCapError struct {
	Window    string
	Cap       int64
	Earned    int64
	Requested int64
}

func (e *ProfitCapError) Error() string {
	return fmt.Sprintf("%s profit limit exceeded: cap=%d earned=%d requested=%d remaining=%d",
		e.Window, e.Cap, e.Earned, e.Requested, e.Cap-e.Earned)
}

func (s *EconomyService) recordProfitCapsTx(ctx context.Context, tx *ent.Tx, u *ent.User, profit int64, now time.Time) error {
	if profit <= 0 {
		return nil
	}

	dailyCap := s.cfg.DailyProfitCap
	weeklyCap := s.cfg.WeeklyProfitCap
	if dailyCap <= 0 && weeklyCap <= 0 {
		return nil
	}

	dailyEarned := u.DailyProfitEarned
	dailyResetAt := u.LastDailyResetAt
	if dailyResetAt.IsZero() || !isSameDay(dailyResetAt, now) {
		dailyEarned = 0
		dailyResetAt = now
	}

	weeklyEarned := u.WeeklyProfitEarned
	weeklyResetAt := u.LastWeeklyResetAt
	if weeklyResetAt.IsZero() || !isSameISOWeek(weeklyResetAt, now) {
		weeklyEarned = 0
		weeklyResetAt = now
	}

	if dailyCap > 0 && dailyEarned+profit > dailyCap {
		return &ProfitCapError{Window: "daily", Cap: dailyCap, Earned: dailyEarned, Requested: profit}
	}
	if weeklyCap > 0 && weeklyEarned+profit > weeklyCap {
		return &ProfitCapError{Window: "weekly", Cap: weeklyCap, Earned: weeklyEarned, Requested: profit}
	}

	updater := tx.User.UpdateOneID(u.ID)
	if dailyCap > 0 || dailyResetAt != u.LastDailyResetAt {
		updater = updater.
			SetDailyProfitEarned(dailyEarned + profit).
			SetLastDailyResetAt(dailyResetAt)
	}
	if weeklyCap > 0 || weeklyResetAt != u.LastWeeklyResetAt {
		updater = updater.
			SetWeeklyProfitEarned(weeklyEarned + profit).
			SetLastWeeklyResetAt(weeklyResetAt)
	}
	if _, err := updater.Save(ctx); err != nil {
		return fmt.Errorf("failed to update profit caps: %w", err)
	}

	u.DailyProfitEarned = dailyEarned + profit
	u.LastDailyResetAt = dailyResetAt
	u.WeeklyProfitEarned = weeklyEarned + profit
	u.LastWeeklyResetAt = weeklyResetAt
	return nil
}

func isSameISOWeek(t1, t2 time.Time) bool {
	y1, w1 := t1.ISOWeek()
	y2, w2 := t2.ISOWeek()
	return y1 == y2 && w1 == w2
}
