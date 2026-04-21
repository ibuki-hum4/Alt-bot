package db

import (
	"context"
	"fmt"

	"alt-bot/ent"
	"alt-bot/internal/config"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

func NewEntClient(lc fx.Lifecycle, cfg config.Config, logger zerolog.Logger) (*ent.Client, error) {
	client, err := ent.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open ent client: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := client.Schema.Create(ctx); err != nil {
				return fmt.Errorf("failed to migrate schema: %w", err)
			}
			logger.Info().Msg("ent client initialized")
			return nil
		},
		OnStop: func(_ context.Context) error {
			if err := client.Close(); err != nil {
				logger.Error().Err(err).Msg("failed to close ent client")
				return err
			}
			return nil
		},
	})

	return client, nil
}
