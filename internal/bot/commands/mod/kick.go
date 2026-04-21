package mod

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handleKick(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, targetID snowflake.ID, reason string) {
	restClient := event.Client().Rest()
	if err := restClient.RemoveMember(guildID, targetID); err != nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("キックに失敗しました: %v", err)).
			SetEphemeral(true).
			Build())
		return
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(fmt.Sprintf("<@%s> をキックしました。理由: %s", targetID.String(), displayReason(reason))).
		SetEphemeral(true).
		Build())
}
