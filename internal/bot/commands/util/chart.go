package util

import (
	"bytes"
	"context"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/rs/zerolog"
)

func HandleChart(logger zerolog.Logger, economy *service.EconomyService, event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	limit := data.Int("limit")
	if limit == 0 {
		limit = 20
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	points, err := economy.GetPriceHistory(ctx, limit)
	if err != nil {
		logger.Error().Err(err).Msg("failed to load price history")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("価格履歴の取得に失敗しました。時間をおいて再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	if len(points) == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("価格履歴がまだありません。").
			SetEphemeral(true).
			Build())
		return
	}

	snapshot, err := economy.GetRateSnapshot(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("failed to load market snapshot")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("市場情報の取得に失敗しました。時間をおいて再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	ascending := make([]service.PricePoint, len(points))
	for i := range points {
		ascending[len(points)-1-i] = points[i]
	}

	markers := make([]ChartEventMarker, 0, 1)
	if snapshot.CurrentEvent != "" && len(ascending) > 0 {
		markers = append(markers, ChartEventMarker{
			Time:  formatJSTClock(ascending[len(ascending)-1].CreatedAt),
			Event: string(snapshot.CurrentEvent),
		})
	}

	pngBytes, err := RenderMarketChartPNG(ascending, snapshot, markers)
	if err != nil {
		logger.Error().Err(err).Msg("failed to render chart png")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("チャート生成に失敗しました。時間をおいて再試行してください。").
			SetEphemeral(true).
			Build())
		return
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent("ALToken Market Chart").
		AddFile("altoken_chart.png", "ALToken market chart", bytes.NewReader(pngBytes)).
		SetEphemeral(true).
		Build())
}
