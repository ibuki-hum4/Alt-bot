package casino

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func HandleCasino(event *events.ApplicationCommandInteractionCreate) {
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
			SetContent("/casino blackjack|chinchiro|roulette|slot|poker を指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	switch *data.SubCommandName {
	case "blackjack":
		handleBlackjack(event, guildID)
	case "chinchiro":
		handleChinchiro(event, guildID)
	case "roulette":
		handleRoulette(event, guildID)
	case "slot":
		handleSlot(event, guildID)
	case "poker":
		handlePoker(event, guildID)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。").
			SetEphemeral(true).
			Build())
	}
}

func sendCasinoPlaceholder(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, title string) {
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitle(title).
			SetDescription("このゲームは現在ベータ実装中です。最小機能を順次追加します。").
			AddField("Guild", guildID.String(), true).
			SetColor(0xF1C40F).
			Build()).
		SetEphemeral(true).
		Build())
}