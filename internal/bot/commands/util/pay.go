package util

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alt-bot/internal/bot/commands/uierr"
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
)

const payOperationTimeout = 5 * time.Second

func HandlePay(logger zerolog.Logger, economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
	if event.GuildID() == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このコマンドはサーバー内でのみ利用できます。").
			SetEphemeral(true).
			Build())
		return
	}

	data := event.SlashCommandInteractionData()
	target := data.User("user")
	if target.ID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("対象ユーザーの指定が不正です。").
			SetEphemeral(true).
			Build())
		return
	}

	fromID := event.User().ID.String()
	toID := target.ID.String()
	if fromID == toID {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("自分自身には送金できません。").
			SetEphemeral(true).
			Build())
		return
	}

	amount := int64(data.Int("amount"))
	if amount <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("amount は 1 以上で指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	note := strings.TrimSpace(data.String("note"))
	if len(note) > 120 {
		note = note[:120]
	}

	ctx, cancel := context.WithTimeout(context.Background(), payOperationTimeout)
	defer cancel()

	res, err := economy.PayYen(ctx, fromID, toID, amount)
	if err != nil {
		message, ok := uierr.Format(err, "受取")
		if !ok {
			logger.Error().Err(err).Msg("pay failed")
			message = "送金に失敗しました。少し待って再試行してください。"
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	embedBuilder := discord.NewEmbedBuilder().
		SetTitle("Pay 完了").
		SetColor(0x2ECC71).
		AddField("送金額", fmt.Sprintf("%d %s", res.Amount, service.CurrencyYenUnit), true).
		AddField("送金元残高", fmt.Sprintf("%d %s", res.FromBalance, service.CurrencyYenUnit), true).
		AddField("送金先残高", fmt.Sprintf("%d %s", res.ToBalance, service.CurrencyYenUnit), true)
	if note != "" {
		embedBuilder = embedBuilder.AddField("メモ", note, false)
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embedBuilder.SetTimestamp(time.Now()).Build()).
		SetEphemeral(true).
		Build())
}
