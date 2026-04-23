package casino

import (
	"context"
	"fmt"
	"strings"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func handleRoulette(event *events.ApplicationCommandInteractionCreate, guildID snowflake.ID, economy *service.EconomyService) {
	betTypeRaw := strings.ToLower(strings.TrimSpace(event.SlashCommandInteractionData().String("bet_type")))
	number := event.SlashCommandInteractionData().Int("number")

	var betType service.RouletteBetType
	switch betTypeRaw {
	case "red":
		betType = service.RouletteBetRed
	case "black":
		betType = service.RouletteBetBlack
	case "odd":
		betType = service.RouletteBetOdd
	case "even":
		betType = service.RouletteBetEven
	case "single":
		betType = service.RouletteBetSingle
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("bet_type は red|black|odd|even|single を指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	if betType == service.RouletteBetSingle && (number < 0 || number > 36) {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("single の場合は number を 0-36 で指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	desc := "中〜高分散モデル。European Roulette準拠の確率で抽選。"
	if betType == service.RouletteBetSingle {
		desc = fmt.Sprintf("中〜高分散モデル。single:%d を抽選。", number)
	}

	runCasinoWeighted(
		event,
		"Roulette",
		desc,
		guildID.String(),
		func(ctx context.Context, discordID string, bet int64) (service.CasinoPlayResult, error) {
			return economy.PlayRoulette(ctx, discordID, bet, betType, number)
		},
	)
}