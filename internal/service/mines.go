package service

import (
	"context"
	"fmt"
)

func (s *EconomyService) MinesPlaceBet(ctx context.Context, discordID string, bet int64) (int64, error) {
	return 0, fmt.Errorf("economy disabled")
}

func (s *EconomyService) MinesCashout(ctx context.Context, discordID string, payout int64) (int64, error) {
	return 0, fmt.Errorf("economy disabled")
}
