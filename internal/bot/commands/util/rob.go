package util

import (
	"context"
	"errors"
	"fmt"
	"time"

	"alt-bot/internal/bot/commands/uierr"
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
)

const robOperationTimeout = 5 * time.Second

func HandleRob(logger zerolog.Logger, economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
	if event.GuildID() == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このコマンドはサーバー内でのみ利用できます。").
			SetEphemeral(true).
			Build())
		return
	}

	data := event.SlashCommandInteractionData()
	target := data.User("target")
	if target.ID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("対象ユーザーの指定が不正です。").
			SetEphemeral(true).
			Build())
		return
	}
	if target.Bot {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Botを対象にすることはできません。").
			SetEphemeral(true).
			Build())
		return
	}

	attackerID := event.User().ID.String()
	targetID := target.ID.String()
	if attackerID == targetID {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("自分自身を対象にすることはできません。").
			SetEphemeral(true).
			Build())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), robOperationTimeout)
	defer cancel()

	res, err := economy.RobUser(ctx, attackerID, targetID)
	if err != nil {
		var cdErr *service.RobCooldownError
		if errors.As(err, &cdErr) {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent(fmt.Sprintf("rob はクールダウン中です。次に実行できる時刻: <t:%d:R>", cdErr.Until.Unix())).
				SetEphemeral(true).
				Build())
			return
		}

		var balErr *service.RobTargetBalanceError
		if errors.As(err, &balErr) {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent(fmt.Sprintf("対象の残高が少なすぎます(必要: %d %s以上)。", balErr.Need, service.CurrencyYenUnit)).
				SetEphemeral(true).
				Build())
			return
		}

		message, ok := uierr.Format(err, "獲得")
		if !ok {
			logger.Error().Err(err).Msg("rob failed")
			message = "強奪に失敗しました。少し待って再試行してください。"
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	attackerMention := fmt.Sprintf("<@%s>", attackerID)
	targetMention := fmt.Sprintf("<@%s>", targetID)

	var embed discord.Embed
	switch {
	case res.Blocked:
		embed = discord.NewEmbedBuilder().
			SetTitle("Rob 失敗 - 防犯カメラ作動").
			SetDescription(fmt.Sprintf("%s は %s を狙いましたが、防犯カメラに阻まれました。対象のカメラが1個消費されました。", attackerMention, targetMention)).
			SetColor(0x95A5A6).
			SetTimestamp(time.Now()).
			Build()
	case res.Success:
		embed = discord.NewEmbedBuilder().
			SetTitle("Rob 成功").
			SetDescription(fmt.Sprintf("%s は %s から %d %s を奪いました!", attackerMention, targetMention, res.Amount, service.CurrencyYenUnit)).
			SetColor(0xE74C3C).
			AddField("攻撃側残高", fmt.Sprintf("%d %s", res.AttackerBalance, service.CurrencyYenUnit), true).
			AddField("対象残高", fmt.Sprintf("%d %s", res.TargetBalance, service.CurrencyYenUnit), true).
			SetTimestamp(time.Now()).
			Build()
	default:
		embed = discord.NewEmbedBuilder().
			SetTitle("Rob 失敗 - 通報").
			SetDescription(fmt.Sprintf("%s は %s への強奪に失敗し、罰金 %d %s を支払いました。", attackerMention, targetMention, res.Amount, service.CurrencyYenUnit)).
			SetColor(0x2ECC71).
			AddField("攻撃側残高", fmt.Sprintf("%d %s", res.AttackerBalance, service.CurrencyYenUnit), true).
			AddField("対象残高", fmt.Sprintf("%d %s", res.TargetBalance, service.CurrencyYenUnit), true).
			SetTimestamp(time.Now()).
			Build()
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		Build())
}
