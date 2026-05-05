package util

import (
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
)

func HandleRolePanel(logger zerolog.Logger, event *events.ApplicationCommandInteractionCreate) {
	embed := discord.NewEmbedBuilder().
		SetTitle("役職パネル").
		SetDescription("このパネルから役職を取得できます。").
		SetColor(0x2ECC71).
		AddField("役職1", "説明1", true).
		AddField("役職2", "説明2", true).
		AddField("役職3", "説明3", true).
		SetTimestamp(time.Now()).
		Build()
	if err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		SetEphemeral(true).
		Build()); err != nil {
		logger.Error().Err(err).Msg("failed to send role panel")
	}
}
