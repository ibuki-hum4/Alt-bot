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

func runCasinoWeighted(
	event *events.ApplicationCommandInteractionCreate,
	title string,
	description string,
	guildIDText string,
	runner func(ctx context.Context, discordID string, bet int64) (service.CasinoPlayResult, error),
) {
	bet := int64(event.SlashCommandInteractionData().Int("amount"))
	if bet <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("amount は 1 以上で指定してください。例: /casino <game> amount:100").
			SetEphemeral(true).
			Build())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := runner(ctx, event.User().ID.String(), bet)
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
			SetContent("カジノ実行中にエラーが発生しました。少し待って再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	resultTitle := title + " 結果"
	color := 0x95A5A6
	if res.NetYen > 0 {
		resultTitle = title + " 勝利"
		color = 0x2ECC71
	} else if res.NetYen < 0 {
		resultTitle = title + " 残念"
		color = 0xE74C3C
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitle(resultTitle).
			SetDescription(description).
			SetColor(color).
			AddField("Result", strings.Join(res.Symbols, " | "), false).
			AddField("倍率", fmt.Sprintf("%.2fx", res.Multiplier), true).
			AddField("Bet", fmt.Sprintf("%d %s", res.BetYen, service.CurrencyYenUnit), true).
			AddField("払戻", fmt.Sprintf("%d %s", res.PayoutYen, service.CurrencyYenUnit), true).
			AddField("収支", fmt.Sprintf("%+d %s", res.NetYen, service.CurrencyYenUnit), true).
			AddField("残りYen", fmt.Sprintf("%d %s", res.YenBalance, service.CurrencyYenUnit), true).
			AddField("Guild", guildIDText, true).
			SetTimestamp(time.Now()).
			Build()).
		SetEphemeral(true).
		Build())
}
