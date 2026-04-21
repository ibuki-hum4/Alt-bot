package crypto

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
)

const (
	cryptoInteractionTimeout = 45 * time.Second
	interactionOpTimeout     = 5 * time.Second
)

func HandleCryptoSlash(logger zerolog.Logger, economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/crypto buy または /crypto sell を指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	op := strings.ToLower(*data.SubCommandName)
	if op != "buy" && op != "sell" {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/crypto buy または /crypto sell を指定してください。").
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

	userID := event.User().ID.String()
	currentPrice := economy.CurrentALTPrice()
	estimate := int64(math.Ceil(currentPrice * float64(amount)))
	if op == "sell" {
		estimate = int64(math.Floor(currentPrice * float64(amount)))
	}

	expiresAt := time.Now().Add(cryptoInteractionTimeout)
	confirmID := buildCryptoConfirmID(userID, op, amount, expiresAt.Unix())
	cancelID := buildCryptoCancelID(userID, expiresAt.Unix())

	actionLabel := "購入"
	if op == "sell" {
		actionLabel = "売却"
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Crypto 取引確認").
		SetDescription(fmt.Sprintf("%s を実行しますか?", actionLabel)).
		SetColor(0x1ABC9C).
		AddField("取引", strings.ToUpper(op), true).
		AddField("数量", fmt.Sprintf("%d %s", amount, service.CurrencyALTUnit), true).
		AddField("現在価格", fmt.Sprintf("%.2f %s/%s", currentPrice, service.CurrencyYenUnit, service.CurrencyALTUnit), true).
		AddField("概算", fmt.Sprintf("%d %s", estimate, service.CurrencyYenUnit), true).
		AddField("有効期限", fmt.Sprintf("<t:%d:R>", expiresAt.Unix()), true).
		Build()

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		AddActionRow(
			discord.NewSuccessButton("確定", confirmID),
			discord.NewSecondaryButton("キャンセル", cancelID),
		).
		SetEphemeral(true).
		Build()); err != nil {
		logger.Error().Err(err).Msg("failed to show crypto confirmation")
		return
	}

	scheduleDisableInteractionButtons(logger, event.Client(), event.Token(), cryptoInteractionTimeout, cryptoDisabledRow(confirmID, cancelID))
}

func HandleCryptoComponent(logger zerolog.Logger, economy *service.EconomyService, event *events.ComponentInteractionCreate) {
	customID := event.Data.CustomID()
	parts := strings.Split(customID, ":")
	if len(parts) < 4 || parts[0] != "crypto" {
		return
	}

	switch parts[1] {
	case "cancel":
		userID, expiresAt, err := parseCryptoCancelID(customID)
		if err != nil {
			return
		}
		if event.User().ID.String() != userID {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("このボタンはコマンド実行者のみ操作できます。").
				SetEphemeral(true).
				Build())
			return
		}
		if time.Now().Unix() > expiresAt {
			embed := discord.NewEmbedBuilder().
				SetTitle("タイムアウト").
				SetDescription("確認の有効期限が切れました。再度 /crypto を実行してください。").
				SetColor(0x95A5A6).
				Build()
			_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().SetEmbeds(embed).ClearContainerComponents().Build())
			return
		}
		embed := discord.NewEmbedBuilder().
			SetTitle("キャンセル").
			SetDescription("取引をキャンセルしました。").
			SetColor(0x95A5A6).
			Build()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().SetEmbeds(embed).ClearContainerComponents().Build())

	case "confirm":
		userID, op, amount, expiresAt, err := parseCryptoConfirmID(customID)
		if err != nil {
			return
		}
		if event.User().ID.String() != userID {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("このボタンはコマンド実行者のみ操作できます。").
				SetEphemeral(true).
				Build())
			return
		}

		confirmID := buildCryptoConfirmID(userID, op, amount, expiresAt)
		cancelID := buildCryptoCancelID(userID, expiresAt)
		if time.Now().Unix() > expiresAt {
			embed := discord.NewEmbedBuilder().
				SetTitle("タイムアウト").
				SetDescription("確認の有効期限が切れました。再度 /crypto を実行してください。").
				SetColor(0x95A5A6).
				Build()
			_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
				SetEmbeds(embed).
				SetContainerComponents(cryptoDisabledRow(confirmID, cancelID)).
				Build())
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), interactionOpTimeout)
		defer cancel()

		if op == "buy" {
			res, err := economy.BuyCrypto(ctx, userID, amount)
			if err != nil {
				updateCryptoError(logger, event, err, confirmID, cancelID)
				return
			}
			embed := discord.NewEmbedBuilder().
				SetTitle("ALToken 購入完了").
				SetColor(0x3498DB).
				AddField("購入枚数", fmt.Sprintf("%d %s", res.Amount, service.CurrencyALTUnit), true).
				AddField("決済単価", fmt.Sprintf("%.2f %s/%s", res.SettledPrice, service.CurrencyYenUnit, service.CurrencyALTUnit), true).
				AddField("支払額", fmt.Sprintf("%d %s", res.TotalCostYen, service.CurrencyYenUnit), true).
				AddField("残りYen", fmt.Sprintf("%d %s", res.YenBalance, service.CurrencyYenUnit), true).
				AddField("保有ALToken", fmt.Sprintf("%d %s", res.AltBalance, service.CurrencyALTUnit), true).
				AddField("現在価格", fmt.Sprintf("%.2f %s/%s", res.CurrentPrice, service.CurrencyYenUnit, service.CurrencyALTUnit), true).
				SetTimestamp(time.Now()).
				Build()
			_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
				SetEmbeds(embed).
				SetContainerComponents(cryptoDisabledRow(confirmID, cancelID)).
				Build())
			return
		}

		res, err := economy.SellCrypto(ctx, userID, amount)
		if err != nil {
			updateCryptoError(logger, event, err, confirmID, cancelID)
			return
		}
		embed := discord.NewEmbedBuilder().
			SetTitle("ALToken 売却完了").
			SetColor(0x2ECC71).
			AddField("売却枚数", fmt.Sprintf("%d %s", res.Amount, service.CurrencyALTUnit), true).
			AddField("決済単価", fmt.Sprintf("%.2f %s/%s", res.SettledPrice, service.CurrencyYenUnit, service.CurrencyALTUnit), true).
			AddField("受取額", fmt.Sprintf("%d %s", res.TotalRevenueYen, service.CurrencyYenUnit), true).
			AddField("残りYen", fmt.Sprintf("%d %s", res.YenBalance, service.CurrencyYenUnit), true).
			AddField("保有ALToken", fmt.Sprintf("%d %s", res.AltBalance, service.CurrencyALTUnit), true).
			AddField("現在価格", fmt.Sprintf("%.2f %s/%s", res.CurrentPrice, service.CurrencyYenUnit, service.CurrencyALTUnit), true).
			SetTimestamp(time.Now()).
			Build()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(embed).
			SetContainerComponents(cryptoDisabledRow(confirmID, cancelID)).
			Build())
	}
}

func cryptoDisabledRow(confirmID string, cancelID string) discord.ContainerComponent {
	confirm := discord.NewSuccessButton("確定", confirmID)
	confirm.Disabled = true
	cancel := discord.NewSecondaryButton("キャンセル", cancelID)
	cancel.Disabled = true
	return discord.NewActionRow(confirm, cancel)
}

func buildCryptoConfirmID(userID string, op string, amount int64, expiresAt int64) string {
	return fmt.Sprintf("crypto:confirm:%s:%s:%d:%d", op, userID, amount, expiresAt)
}

func parseCryptoConfirmID(customID string) (string, string, int64, int64, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 6 || parts[0] != "crypto" || parts[1] != "confirm" {
		return "", "", 0, 0, fmt.Errorf("invalid crypto confirm id")
	}
	op := parts[2]
	if op != "buy" && op != "sell" {
		return "", "", 0, 0, fmt.Errorf("invalid crypto op")
	}
	amount, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return "", "", 0, 0, err
	}
	expiresAt, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil {
		return "", "", 0, 0, err
	}
	return parts[3], op, amount, expiresAt, nil
}

func buildCryptoCancelID(userID string, expiresAt int64) string {
	return fmt.Sprintf("crypto:cancel:%s:%d", userID, expiresAt)
}

func parseCryptoCancelID(customID string) (string, int64, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 4 || parts[0] != "crypto" || parts[1] != "cancel" {
		return "", 0, fmt.Errorf("invalid crypto cancel id")
	}
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return parts[2], expiresAt, nil
}

func updateCryptoError(logger zerolog.Logger, event *events.ComponentInteractionCreate, err error, confirmID string, cancelID string) {
	desc := "処理中にエラーが発生しました。時間をおいて再試行してください。"
	var yenErr *service.InsufficientYenError
	if errors.As(err, &yenErr) {
		desc = fmt.Sprintf("Yenが不足しています。必要: %d / 現在: %d", yenErr.Need, yenErr.Have)
	} else {
		var altErr *service.InsufficientALTError
		if errors.As(err, &altErr) {
			desc = fmt.Sprintf("ALTokenが不足しています。必要: %d / 現在: %d", altErr.Need, altErr.Have)
		} else {
			var haltedErr *service.MarketHaltedError
			if errors.As(err, &haltedErr) {
				desc = "市場は緊急停止中です。30分安定後に自動解除されます。"
			} else {
				var limitErr *service.CircuitLimitError
				if errors.As(err, &limitErr) {
					desc = fmt.Sprintf("サーキット制限により注文上限を超えました。現在の上限: %d ALT", limitErr.MaxQty)
				} else {
					var issuanceErr *service.DailyIssuanceCapError
					if errors.As(err, &issuanceErr) {
						desc = fmt.Sprintf("本日の発行上限に達しました。上限: %d / 既発行: %d", issuanceErr.Cap, issuanceErr.Issued)
					} else {
						logger.Error().Err(err).Msg("crypto confirm failed")
					}
				}
			}
		}
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("取引エラー").
		SetDescription(desc).
		SetColor(0xE74C3C).
		Build()

	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(embed).
		SetContainerComponents(cryptoDisabledRow(confirmID, cancelID)).
		Build())
}

func scheduleDisableInteractionButtons(logger zerolog.Logger, client bot.Client, token string, d time.Duration, row discord.ContainerComponent) {
	go func() {
		t := time.NewTimer(d)
		defer t.Stop()
		<-t.C

		_, err := client.Rest().UpdateInteractionResponse(
			client.ApplicationID(),
			token,
			discord.NewMessageUpdateBuilder().SetContainerComponents(row).Build(),
		)
		if err != nil {
			logger.Debug().Err(err).Msg("failed to disable expired interaction buttons")
		}
	}()
}
