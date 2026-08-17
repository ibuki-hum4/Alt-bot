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
	"github.com/disgoorg/disgo/gateway"
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
			service.NewRolePanelService,
			service.NewStickyService,
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
	opts := []dbot.ConfigOpt{
		dbot.WithDefaultGateway(),
		dbot.WithEventListenerFunc(func(event dbot.Event) {
			switch e := event.(type) {
			case *events.ApplicationCommandInteractionCreate:
				handlers.OnApplicationCommandInteraction(e)
			case *events.AutocompleteInteractionCreate:
				handlers.OnAutocompleteInteraction(e)
			case *events.ComponentInteractionCreate:
				handlers.OnComponentInteraction(e)
			case *events.ModalSubmitInteractionCreate:
				handlers.OnModalSubmit(e)
			case *events.MessageCreate:
				handlers.OnMessageCreate(e)
			}
		}),
	}

	// Interactions arrive without any gateway intent, so the bot otherwise runs
	// with none at all. Sticky messages are the only feature that needs to see
	// ordinary messages, so the intent is requested only when it is enabled.
	// IntentGuildMessages is not privileged; the privileged message content
	// intent stays off because only the fact a message arrived matters here.
	if cfg.StickyEnabled {
		opts = append(opts, dbot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages)))
		logger.Info().Msg("guild message intent requested for sticky messages")
	}

	client, err := disgo.New(cfg.DiscordToken, opts...)
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
			logFeatureFlags(logger, handlers)
			handlers.StartNewsLoop(client)
			handlers.StartSticky(ctx, client)
			logger.Info().Msg("bot started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			handlers.StopNewsLoop()
			handlers.StopStickyTimers()
			client.Close(ctx)
			logger.Info().Msg("bot stopped")
			return nil
		},
	})
}

func logFeatureFlags(logger zerolog.Logger, handlers *ibot.Handlers) {
	cfg := handlers.Config()
	logger.Info().Msgf(
			"[Feature Flags]\nEconomy: %s\nCasino: %s\nCrypto: %s\nRolePanel: %s\nModeration: %s\nSticky: %s",
		featureState(cfg.EconomyEnabled),
		featureState(cfg.CasinoEnabled),
		featureState(cfg.CryptoEnabled),
		featureState(cfg.RolePanelEnabled),
		featureState(cfg.ModEnabled),
		featureState(cfg.StickyEnabled),
	)
}

func featureState(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
