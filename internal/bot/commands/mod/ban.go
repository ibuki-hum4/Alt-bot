package mod

import (
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handleBan(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, targetID snowflake.ID, deleteDays int, reason string) {
	if deleteDays < 0 {
		deleteDays = 0
	}
	if deleteDays > 7 {
		deleteDays = 7
	}

	restClient := event.Client().Rest()
	if err := restClient.AddBan(guildID, targetID, time.Duration(deleteDays)*24*time.Hour); err != nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("BANに失敗しました: %v", err)).
			SetEphemeral(true).
			Build())
		return
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(fmt.Sprintf("<@%s> をBANしました。理由: %s", targetID.String(), displayReason(reason))).
		SetEphemeral(true).
		Build())
}
