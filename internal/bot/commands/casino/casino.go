package casino

import (
	"strings"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func HandleCasino(economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
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
			SetContent("/casino blackjack|chinchiro|mines|poker を指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	sub := strings.ToLower(*data.SubCommandName)
	switch sub {
	case "blackjack":
		handleBlackjack(event, guildID, economy)
	case "chinchiro":
		handleChinchiro(event, guildID, economy)
	case "mines":
		handleMines(event, guildID, economy)
	case "poker":
		HandlePoker(event, economy)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。").
			SetEphemeral(true).
			Build())
	}
}
