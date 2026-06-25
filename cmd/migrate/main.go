package main

import (
	"context"
	"fmt"
	"os"

	"alt-bot/internal/config"
	"alt-bot/internal/db"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	level, parseErr := zerolog.ParseLevel(cfg.LogLevel)
	if parseErr != nil {
		level = zerolog.InfoLevel
	}
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger().Level(level)

	client, err := db.OpenEntClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}
	defer client.Close()

	if err := db.MigrateSchema(context.Background(), client); err != nil {
		log.Fatal().Err(err).Msg("schema migration failed")
	}

	logger.Info().Msg("schema migration completed")
	fmt.Println("schema migration completed")
}