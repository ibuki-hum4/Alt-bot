package mod

import (
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func HandleModeration(event *events.ApplicationCommandInteractionCreate) {
	if event.GuildID() == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このコマンドはサーバー内でのみ利用できます。").
			SetEphemeral(true).
			Build())
		return
	}

	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。").
			SetEphemeral(true).
			Build())
		return
	}

	sub := strings.ToLower(*data.SubCommandName)
	target := data.User("user")
	if target.ID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("対象ユーザーの指定が不正です。").
			SetEphemeral(true).
			Build())
		return
	}

	actorID := event.User().ID
	if target.ID == actorID {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("自分自身には実行できません。").
			SetEphemeral(true).
			Build())
		return
	}

	reason := data.String("reason")
	guildID := *event.GuildID()

	switch sub {
	case "kick":
		handleKick(event, guildID, target.ID, reason)
	case "ban":
		handleBan(event, guildID, target.ID, data.Int("delete_days"), reason)
	case "mute":
		handleMute(event, guildID, target.ID, data.Int("minutes"), reason)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。").
			SetEphemeral(true).
			Build())
	}
}

func displayReason(reason string) string {
	r := strings.TrimSpace(reason)
	if r == "" {
		return "なし"
	}
	return r
}
