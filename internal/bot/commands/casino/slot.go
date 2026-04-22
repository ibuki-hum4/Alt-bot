package casino

import (
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handleSlot(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID) {
	sendCasinoPlaceholder(event, guildID, "Casino: Slot")
}