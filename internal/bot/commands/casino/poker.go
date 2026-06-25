package casino

import (
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handlePoker(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	_ = economy
	startCasinoSession(
		event,
		"poker",
		"Poker",
		"手札の強さで結果が決まります。",
		guildID.String(),
		"",
	)
}
