package util

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"alt-bot/internal/bot/commands/uierr"
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
)

const (
	shopOperationTimeout   = 5 * time.Second
	shopInteractionTimeout = 60 * time.Second

	// maxShopDisplayItems mirrors Discord's hard limits: at most 25 fields
	// per embed and at most 25 options per string select menu. Both the list
	// embed and the select menu must stay within this, or the API rejects
	// the whole message.
	maxShopDisplayItems = 25
)

// shopDisplayItems caps the catalog to what Discord can actually render in
// one embed/select menu, so adding shop items can never silently break /shop.
func shopDisplayItems(items []service.ShopItem) (display []service.ShopItem, truncated bool) {
	if len(items) <= maxShopDisplayItems {
		return items, false
	}
	return items[:maxShopDisplayItems], true
}

func HandleShop(logger zerolog.Logger, economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
	items := economy.ShopItems()
	if len(items) == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("現在はまだ商品が登録されていません。").
			SetEphemeral(true).
			Build())
		return
	}

	displayItems, truncated := shopDisplayItems(items)

	userID := event.User().ID.String()
	expiresAt := time.Now().Add(shopInteractionTimeout).Unix()

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(buildShopListEmbed(displayItems, len(items), truncated, expiresAt)).
		SetContainerComponents(buildShopSelectRow(displayItems, userID, expiresAt)).
		SetEphemeral(true).
		Build()); err != nil {
		logger.Error().Err(err).Msg("failed to show shop list")
	}
}

func HandleShopComponent(logger zerolog.Logger, economy *service.EconomyService, event *events.ComponentInteractionCreate) {
	parts := strings.Split(event.Data.CustomID(), ":")
	if len(parts) < 4 || parts[0] != "shop" {
		return
	}
	action := parts[1]
	userID := parts[2]

	if event.User().ID.String() != userID {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このメニューはコマンド実行者のみ操作できます。").
			SetEphemeral(true).
			Build())
		return
	}

	switch action {
	case "select":
		handleShopSelect(economy, event, parts)
	case "confirm":
		handleShopConfirm(logger, economy, event, parts)
	case "cancel":
		handleShopCancel(event)
	}
}

func handleShopSelect(economy *service.EconomyService, event *events.ComponentInteractionCreate, parts []string) {
	if len(parts) != 4 {
		return
	}
	userID := parts[2]
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return
	}
	if time.Now().Unix() > expiresAt {
		respondShopTimeout(event)
		return
	}

	values := event.StringSelectMenuInteractionData().Values
	if len(values) == 0 {
		return
	}
	itemID := values[0]

	item, ok := economy.ShopItemByID(itemID)
	if !ok {
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(discord.NewEmbedBuilder().
				SetTitle("Shop エラー").
				SetDescription("商品が見つかりません。再度 /shop を実行してください。").
				SetColor(0xE74C3C).
				Build()).
			ClearContainerComponents().
			Build())
		return
	}

	confirmID := buildShopConfirmID(userID, itemID, expiresAt)
	cancelID := buildShopCancelID(userID, expiresAt)

	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(buildShopConfirmEmbed(item)).
		SetContainerComponents(discord.NewActionRow(
			discord.NewSuccessButton("購入する", confirmID),
			discord.NewSecondaryButton("キャンセル", cancelID),
		)).
		Build())
}

func handleShopConfirm(logger zerolog.Logger, economy *service.EconomyService, event *events.ComponentInteractionCreate, parts []string) {
	if len(parts) != 5 {
		return
	}
	userID := parts[2]
	itemID := parts[3]
	expiresAt, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return
	}
	if time.Now().Unix() > expiresAt {
		respondShopTimeout(event)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), shopOperationTimeout)
	defer cancel()

	res, buyErr := economy.BuyShopItem(ctx, userID, itemID, 1)
	if buyErr != nil {
		message, ok := uierr.Format(buyErr, "獲得")
		if !ok {
			logger.Error().Err(buyErr).Msg("shop buy failed")
			message = "商品購入に失敗しました。少し待って再試行してください。"
		}
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(discord.NewEmbedBuilder().
				SetTitle("購入エラー").
				SetDescription(message).
				SetColor(0xE74C3C).
				Build()).
			ClearContainerComponents().
			Build())
		return
	}

	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(buildShopPurchaseResultEmbed(res)).
		ClearContainerComponents().
		Build())
}

func handleShopCancel(event *events.ComponentInteractionCreate) {
	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitle("Shop キャンセル").
			SetDescription("購入をキャンセルしました。再度 /shop を実行してください。").
			SetColor(0x95A5A6).
			Build()).
		ClearContainerComponents().
		Build())
}

func respondShopTimeout(event *events.ComponentInteractionCreate) {
	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitle("Shop タイムアウト").
			SetDescription("操作の有効期限が切れました。再度 /shop を実行してください。").
			SetColor(0x95A5A6).
			Build()).
		ClearContainerComponents().
		Build())
}

func buildShopListEmbed(displayItems []service.ShopItem, totalCount int, truncated bool, expiresAt int64) discord.Embed {
	builder := discord.NewEmbedBuilder().
		SetTitle("Shop").
		SetDescription(fmt.Sprintf("下のメニューから購入したい商品を選択してください。\n有効期限: <t:%d:R>", expiresAt)).
		SetColor(0x9B59B6).
		SetTimestamp(time.Now())
	for _, item := range displayItems {
		limit := "無制限"
		if item.MaxQuantity > 0 {
			limit = fmt.Sprintf("%d", item.MaxQuantity)
		}
		builder.AddField(
			fmt.Sprintf("%s (%s)", item.Name, item.ID),
			fmt.Sprintf("価格: %d %s / 購入上限: %s / %s", item.Price, service.CurrencyYenUnit, limit, item.Description),
			false,
		)
	}
	if truncated {
		builder.SetFooterText(fmt.Sprintf("表示上限のため %d 件中 %d 件のみ表示しています。", totalCount, len(displayItems)))
	}
	return builder.Build()
}

func buildShopSelectRow(items []service.ShopItem, userID string, expiresAt int64) discord.ContainerComponent {
	options := make([]discord.StringSelectMenuOption, 0, len(items))
	for _, item := range items {
		options = append(options, discord.NewStringSelectMenuOption(item.Name, item.ID).
			WithDescription(fmt.Sprintf("%d %s", item.Price, service.CurrencyYenUnit)))
	}
	menu := discord.NewStringSelectMenu(buildShopSelectID(userID, expiresAt), "購入する商品を選択", options...)
	return discord.NewActionRow(menu)
}

func buildShopConfirmEmbed(item service.ShopItem) discord.Embed {
	return discord.NewEmbedBuilder().
		SetTitle("購入確認").
		SetDescription(fmt.Sprintf("「%s」を購入しますか?", item.Name)).
		SetColor(0xF39C12).
		AddField("価格", fmt.Sprintf("%d %s", item.Price, service.CurrencyYenUnit), true).
		AddField("説明", item.Description, false).
		SetTimestamp(time.Now()).
		Build()
}

func buildShopPurchaseResultEmbed(res service.ShopPurchaseResult) discord.Embed {
	builder := discord.NewEmbedBuilder().
		SetTitle("Shop 購入完了").
		SetColor(0x2ECC71).
		AddField("商品", res.Item.Name, true).
		AddField("数量", fmt.Sprintf("%d", res.Quantity), true).
		AddField("支払額", fmt.Sprintf("%d %s", res.TotalPrice, service.CurrencyYenUnit), true).
		AddField("残りYen", fmt.Sprintf("%d %s", res.BalanceAfter, service.CurrencyYenUnit), true).
		SetTimestamp(time.Now())
	if res.WorkReset {
		builder.AddField("効果", "Workクールダウン解除", false)
	}
	if res.XpGain > 0 {
		builder.AddField("XP", fmt.Sprintf("+%d", res.XpGain), true)
	}
	if res.AltGain > 0 {
		builder.AddField("ALToken", fmt.Sprintf("+%d %s", res.AltGain, service.CurrencyALTUnit), true)
	}
	if res.CameraGain > 0 {
		builder.AddField("防犯カメラ", fmt.Sprintf("+%d 個", res.CameraGain), true)
	}
	return builder.Build()
}

func buildShopSelectID(userID string, expiresAt int64) string {
	return fmt.Sprintf("shop:select:%s:%d", userID, expiresAt)
}

func buildShopConfirmID(userID string, itemID string, expiresAt int64) string {
	return fmt.Sprintf("shop:confirm:%s:%s:%d", userID, itemID, expiresAt)
}

func buildShopCancelID(userID string, expiresAt int64) string {
	return fmt.Sprintf("shop:cancel:%s:%d", userID, expiresAt)
}
