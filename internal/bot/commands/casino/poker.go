package casino

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func HandlePoker(event *events.ApplicationCommandInteractionCreate, economy *service.EconomyService) {
	bet := int64(event.SlashCommandInteractionData().Int("amount"))
	if bet <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("amount は 1 以上で指定してください。例: /casino poker amount:100").
			SetEphemeral(true).
			Build())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := economy.PlayPoker(ctx, event.User().ID.String(), bet)
	if err != nil {
		var insufficient *service.InsufficientYenError
		if errors.As(err, &insufficient) {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent(fmt.Sprintf("Yen不足です。必要: %d %s / 現在: %d %s", insufficient.Need, service.CurrencyYenUnit, insufficient.Have, service.CurrencyYenUnit)).
				SetEphemeral(true).
				Build())
			return
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ポーカーの実行に失敗しました。少し待って再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	color := 0x95A5A6
	title := "Poker 結果"
	if res.NetYen > 0 {
		title = "Poker 勝利"
		color = 0x2ECC71
	} else if res.NetYen < 0 {
		title = "Poker 残念"
		color = 0xE74C3C
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(title).
		SetDescription("ポーカーの結果").
		SetColor(color).
		AddField("Result", strings.Join(res.Symbols, " | "), false).
		AddField("倍率", fmt.Sprintf("%.2fx", res.Multiplier), true).
		AddField("Bet", fmt.Sprintf("%d %s", res.BetYen, service.CurrencyYenUnit), true).
		AddField("払戻", fmt.Sprintf("%d %s", res.PayoutYen, service.CurrencyYenUnit), true).
		AddField("収支", fmt.Sprintf("%+d %s", res.NetYen, service.CurrencyYenUnit), true).
		AddField("残りYen", fmt.Sprintf("%d %s", res.YenBalance, service.CurrencyYenUnit), true).
		SetTimestamp(time.Now()).
		Build()

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		SetEphemeral(true).
		Build())
}
