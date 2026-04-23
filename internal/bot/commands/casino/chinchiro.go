package casino

import (
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handleChinchiro(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	runCasinoWeighted(
		event,
		"Chinchiro",
		"中分散モデル。強役は低確率で高配当。",
		guildID.String(),
		economy.PlayChinchiro,
	)
}