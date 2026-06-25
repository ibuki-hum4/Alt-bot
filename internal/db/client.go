package db

import (
	"context"
	"fmt"
	"time"

	"alt-bot/ent"
	"alt-bot/internal/config"

	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const (
	maxOpenConns    = 20
	maxIdleConns    = 5
	connMaxLifetime = 30 * time.Minute
)

func OpenEntClient(cfg config.Config) (*ent.Client, error) {
	drv, err := entsql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open ent client: %w", err)
	}

	db := drv.DB()
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	return ent.NewClient(ent.Driver(drv)), nil
}

func NewEntClient(lc fx.Lifecycle, cfg config.Config, logger zerolog.Logger) (*ent.Client, error) {
	client, err := OpenEntClient(cfg)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := MigrateSchema(ctx, client); err != nil {
				return err
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
