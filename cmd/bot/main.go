package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	ibot "alt-bot/internal/bot"
	"alt-bot/internal/config"
	"alt-bot/internal/db"
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo"
	dbot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,
			newOwnerIDs,
			newLogger,
			db.NewEntClient,
			service.NewEconomyService,
			ibot.NewHandlers,
			newDisgoClient,
		),
		fx.Invoke(registerLifecycle),
	).Run()
}

func newOwnerIDs(cfg config.Config) []string {
	return cfg.OwnerIDs
}

func newLogger(cfg config.Config) zerolog.Logger {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.LogLevel))
	if err != nil {
		level = zerolog.InfoLevel
	}
	writer := diode.NewWriter(os.Stdout, 1024, 10*time.Millisecond, func(missed int) {
		_, _ = fmt.Fprintf(os.Stderr, "zerolog dropped %d messages\n", missed)
	})
	logger := zerolog.New(writer).With().Timestamp().Logger().Level(level)
	return logger
}

func newDisgoClient(cfg config.Config, handlers *ibot.Handlers, logger zerolog.Logger) (dbot.Client, error) {
	client, err := disgo.New(cfg.DiscordToken,
		dbot.WithDefaultGateway(),
		dbot.WithEventListenerFunc(func(event dbot.Event) {
			switch e := event.(type) {
			case *events.ApplicationCommandInteractionCreate:
				handlers.OnApplicationCommandInteraction(e)
			case *events.ComponentInteractionCreate:
				handlers.OnComponentInteraction(e)
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create disgo client: %w", err)
	}

	logger.Info().Msg("disgo client created")
	return client, nil
}

func registerLifecycle(lc fx.Lifecycle, client dbot.Client, handlers *ibot.Handlers, logger zerolog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := handlers.RegisterCommands(ctx, client); err != nil {
				return err
			}
			if err := client.OpenGateway(ctx); err != nil {
				return fmt.Errorf("failed to open discord gateway: %w", err)
			}
			handlers.StartNewsLoop(client)
			logger.Info().Msg("bot started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			handlers.StopNewsLoop()
			client.Close(ctx)
			logger.Info().Msg("bot stopped")
			return nil
		},
	})
}
