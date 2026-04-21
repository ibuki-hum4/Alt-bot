package util

import (
	"context"
	"fmt"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func HandleRate(event *events.ApplicationCommandInteractionCreate, economy *service.EconomyService) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshot, err := economy.GetRateSnapshot(ctx)
	if err != nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("市場情報の取得に失敗しました。時間をおいて再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	changeText := "N/A"
	if snapshot.Has24h {
		changeText = fmt.Sprintf("%.2f%%", snapshot.Change24h)
	}
	currentEvent := "NONE"
	if snapshot.CurrentEvent != "" {
		currentEvent = string(snapshot.CurrentEvent)
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("ALToken Rate").
		SetDescription("現在の仮想市場スナップショット").
		SetColor(0x2980B9).
		AddField("現在価格", fmt.Sprintf("%.2f %s/%s", snapshot.CurrentPrice, service.CurrencyYenUnit, service.CurrencyALTUnit), true).
		AddField("24h変動率", changeText, true).
		AddField("現在イベント", currentEvent, true).
		AddField("サーキット", string(snapshot.CircuitLevel), true).
		AddField("安定化リザーブ", fmt.Sprintf("%d %s", snapshot.ReserveYen, service.CurrencyYenUnit), true).
		AddField("運営収益", fmt.Sprintf("%d %s", snapshot.RevenueYen, service.CurrencyYenUnit), true).
		AddField("累計バーン", fmt.Sprintf("%d %s", snapshot.BurnedTotal, service.CurrencyYenUnit), true).
		AddField("市場コメント", snapshot.Comment, false).
		SetTimestamp(snapshot.UpdatedAt).
		Build()

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		SetEphemeral(true).
		Build())
}
