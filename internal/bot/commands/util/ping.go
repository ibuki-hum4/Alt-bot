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

	// Respond with a single CreateMessage call instead of Defer+Update: the
	// embed only needs values we already have (gateway heartbeat latency,
	// time since Discord created the interaction), so deferring would just
	// add a second Discord API round-trip for no benefit.
	if err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitle("Pong").
			SetDescription("レイテンシ計測").
			SetColor(0x3498DB).
			AddField("WebSocket", formatLatencyMS(websocketLatency), true).
			AddField("Command受信", formatLatencyMS(commandReceivedLatency), true).
			SetTimestamp(now).
			Build()).
		Build(),
	); err != nil {
		logger.Error().Err(err).Msg("failed to send ping response")
		return
	}
}
