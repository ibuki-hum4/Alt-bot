package casino

import (
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handleBlackjack(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	runCasinoWeighted(
		event,
		"Blackjack",
		"低分散モデル。Pushを多めに入れて体感を安定化。",
		guildID.String(),
		economy.PlayBlackjack,
	)
}