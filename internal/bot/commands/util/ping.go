package util

import (
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
)

func formatLatencyMS(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	ms := d.Milliseconds()
	if ms == 0 && d > 0 {
		ms = 1
	}
	return fmt.Sprintf("%d ms", ms)
}

func HandlePing(logger zerolog.Logger, event *events.ApplicationCommandInteractionCreate) {
	now := time.Now()

	commandReceivedLatency := now.Sub(event.ID().Time())
	if commandReceivedLatency < 0 {
		commandReceivedLatency = 0
	}

	websocketLatency := time.Duration(0)
	if event.Client().HasGateway() {
		websocketLatency = event.Client().Gateway().Latency()
		if websocketLatency < 0 {
			websocketLatency = 0
		}
	}

	deferStart := time.Now()
	if err := event.DeferCreateMessage(false); err != nil {
		logger.Error().Err(err).Msg("failed to defer ping response")
		return
	}
	responseAPILatency := time.Since(deferStart)

	embed := discord.NewEmbedBuilder().
		SetTitle("Pong").
		SetDescription("レイテンシ計測").
		SetColor(0x3498DB).
		AddField("WebSocket", formatLatencyMS(websocketLatency), true).
		AddField("Command受信", formatLatencyMS(commandReceivedLatency), true).
		AddField("応答API", formatLatencyMS(responseAPILatency), true).
		SetTimestamp(now).
		Build()

	if _, err := event.Client().Rest().UpdateInteractionResponse(
		event.ApplicationID(),
		event.Token(),
		discord.NewMessageUpdateBuilder().
			SetEmbeds(embed).
			Build(),
	); err != nil {
		logger.Error().Err(err).Msg("failed to update ping response")
	}
}
