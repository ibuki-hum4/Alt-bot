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

const shopOperationTimeout = 5 * time.Second

func HandleShop(logger zerolog.Logger, economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("サブコマンドを指定してください。 (list/buy)").
			SetEphemeral(true).
			Build())
		return
	}

	switch strings.ToLower(*data.SubCommandName) {
	case "list":
		items := economy.ShopItems()
		embed := discord.NewEmbedBuilder().
			SetTitle("Shop").
			SetDescription("購入可能な商品一覧です。内容は後から追加・変更できます。").
			SetColor(0x9B59B6).
			SetTimestamp(time.Now())
		if len(items) == 0 {
			embed.AddField("商品", "現在はまだ商品が登録されていません。", false)
		} else {
			for _, item := range items {
				limit := "無制限"
				if item.MaxQuantity > 0 {
					limit = fmt.Sprintf("%d", item.MaxQuantity)
				}
				embed.AddField(
					fmt.Sprintf("%s (%s)", item.Name, item.ID),
					fmt.Sprintf("価格: %d %s / 上限: %s / %s", item.Price, service.CurrencyYenUnit, limit, item.Description),
					false,
				)
			}
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEmbeds(embed.Build()).
			SetEphemeral(true).
			Build())
	case "buy":
		itemID := strings.TrimSpace(data.String("item"))
		if itemID == "" {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("item を指定してください。").
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

		ctx, cancel := context.WithTimeout(context.Background(), shopOperationTimeout)
		defer cancel()

		res, err := economy.BuyShopItem(ctx, event.User().ID.String(), itemID, amount)
		if err != nil {
			message, ok := uierr.Format(err, "獲得")
			if !ok {
				logger.Error().Err(err).Msg("shop buy failed")
				message = "商品購入に失敗しました。少し待って再試行してください。"
			}
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent(message).
				SetEphemeral(true).
				Build())
			return
		}

		embed := discord.NewEmbedBuilder().
			SetTitle("Shop 購入完了").
			SetColor(0x2ECC71).
			AddField("商品", res.Item.Name, true).
			AddField("数量", fmt.Sprintf("%d", res.Quantity), true).
			AddField("支払額", fmt.Sprintf("%d %s", res.TotalPrice, service.CurrencyYenUnit), true).
			AddField("残りYen", fmt.Sprintf("%d %s", res.BalanceAfter, service.CurrencyYenUnit), true)
		if res.WorkReset {
			embed.AddField("効果", "Workクールダウン解除", false)
		}
		if res.XpGain > 0 {
			embed.AddField("XP", fmt.Sprintf("+%d", res.XpGain), true)
		}
		if res.AltGain > 0 {
			embed.AddField("ALToken", fmt.Sprintf("+%d %s", res.AltGain, service.CurrencyALTUnit), true)
		}
		if res.CameraGain > 0 {
			embed.AddField("防犯カメラ", fmt.Sprintf("+%d 個", res.CameraGain), true)
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEmbeds(embed.Build()).
			SetEphemeral(true).
			Build())
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。 (list/buy)").
			SetEphemeral(true).
			Build())
	}
}
