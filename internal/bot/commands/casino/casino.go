package casino

import (
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func HandleCasino(economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
	// 経済機能無効化: 即時応答して何もしない
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent("経済機能は現在無効化されています。/casino は利用できません。").
		SetEphemeral(true).
		Build())
	return
	if event.GuildID() == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このコマンドはサーバー内でのみ利用できます。").
			SetEphemeral(true).
			Build())
		return
	}

	guildID := *event.GuildID()

	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/casino blackjack|chinchiro|mines を指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	switch *data.SubCommandName {
	case "blackjack":
		handleBlackjack(event, guildID, economy)
	case "chinchiro":
		handleChinchiro(event, guildID, economy)
	case "mines":
		handleMines(event, guildID, economy)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。").
			SetEphemeral(true).
			Build())
	}
}
