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
		AddField("Utility", "/help, /ping, /news channel|off|status, /rate, /chart [limit], /pin set|off|status", false).
		AddField("Economy", "/work, /rob target, /shop(選択式), /crypto buy|sell amount", false).
		AddField("Casino", "/casino blackjack|chinchiro|mines|poker amount", false).
		AddField("Moderation", "/mod kick|ban|mute user ...", false).
		AddField("Admin", "/commands reload (Bot Owner only)", false).
		AddField("Role Panel", "/rp create, /rp add, /rp delete", false).
		AddField("右クリックメニュー", "メッセージを右クリック → アプリ → 「固定メッセージに設定」で、既存のメッセージをそのまま固定できます。", false).
		AddField("Notes", "mod と commands は権限設定により表示/実行制限があります。", false).
		Build()

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		SetEphemeral(true).
		Build())
}
