package util

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func HandleHelp(event *events.ApplicationCommandInteractionCreate) {
	embed := discord.NewEmbedBuilder().
		SetTitle("Help").
		SetDescription("利用できるコマンド一覧です").
		SetColor(0x5865F2).
		AddField("Utility", "/help, /ping, /news channel|off|status, /rate, /chart [limit]", false).
		AddField("Economy", "/work, /crypto buy|sell amount", false).
		AddField("Moderation", "/mod kick|ban|mute user ...", false).
		AddField("Admin", "/commands reload (Bot Owner only)", false).
		AddField("Notes", "mod と commands は権限設定により表示/実行制限があります。", false).
		Build()

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		SetEphemeral(true).
		Build())
}
