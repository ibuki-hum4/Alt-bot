package casino

import (
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handleChinchiro(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	_ = economy
	startCasinoSession(
		event,
		"chinchiro",
		"Chinchiro",
		"出目を画像で表示します。",
		guildID.String(),
		"",
	)
}
