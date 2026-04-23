package casino

import (
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handlePoker(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	runCasinoWeighted(
		event,
		"Poker",
		"高分散モデル。低頻度の高配当役を含む。",
		guildID.String(),
		economy.PlayPoker,
	)
}
