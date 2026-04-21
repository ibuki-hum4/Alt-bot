package mod

import (
	"fmt"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	disgojson "github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
)

func handleMute(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, targetID snowflake.ID, minutes int, reason string) {
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 10080 {
		minutes = 10080
	}

	until := time.Now().Add(time.Duration(minutes) * time.Minute).UTC()
	restClient := event.Client().Rest()
	_, err := restClient.UpdateMember(guildID, targetID, discord.MemberUpdate{
		CommunicationDisabledUntil: disgojson.NewNullablePtr(until),
	})
	if err != nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("ミュート(タイムアウト)に失敗しました: %v", err)).
			SetEphemeral(true).
			Build())
		return
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(fmt.Sprintf("<@%s> を %d 分ミュートしました。理由: %s", targetID.String(), minutes, displayReason(reason))).
		SetEphemeral(true).
		Build())
}
