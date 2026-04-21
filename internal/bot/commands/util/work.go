package util

import (
	"context"
	"errors"
	"fmt"
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
	workInteractionTimeout = 45 * time.Second
	interactionOpTimeout   = 5 * time.Second
)

func HandleWorkSlash(logger zerolog.Logger, event *events.ApplicationCommandInteractionCreate) {
	userID := event.User().ID.String()
	expiresAt := time.Now().Add(workInteractionTimeout)
	easyID := buildWorkButtonID(userID, service.WorkDifficultyEasy, expiresAt.Unix())
	hardID := buildWorkButtonID(userID, service.WorkDifficultyHard, expiresAt.Unix())

	embed := discord.NewEmbedBuilder().
		SetTitle("Work: 難易度を選択").
		SetDescription(fmt.Sprintf("Easy / Hard を選んでください。\nこのボタンは <t:%d:R> にタイムアウトします。", expiresAt.Unix())).
		SetColor(0x9B59B6).
		AddField("Easy", "約30-80 ¥", true).
		AddField("Hard", "約120-260 ¥", true).
		Build()

	if err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		AddActionRow(
			discord.NewPrimaryButton("Easy", easyID),
			discord.NewDangerButton("Hard", hardID),
		).
		SetEphemeral(true).
		Build()); err != nil {
		logger.Error().Err(err).Msg("failed to show work buttons")
		return
	}

	scheduleDisableInteractionButtons(logger, event.Client(), event.Token(), workInteractionTimeout, workDisabledRow(easyID, hardID))
}

func HandleWorkComponent(logger zerolog.Logger, economy *service.EconomyService, event *events.ComponentInteractionCreate) {
	userID, difficulty, expiresAt, err := parseWorkButtonID(event.Data.CustomID())
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

	easyID := buildWorkButtonID(userID, service.WorkDifficultyEasy, expiresAt)
	hardID := buildWorkButtonID(userID, service.WorkDifficultyHard, expiresAt)

	if time.Now().Unix() > expiresAt {
		embed := discord.NewEmbedBuilder().
			SetTitle("タイムアウト").
			SetDescription("Work選択の有効期限が切れました。再度 /work を実行してください。").
			SetColor(0x95A5A6).
			Build()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(embed).
			SetContainerComponents(workDisabledRow(easyID, hardID)).
			Build())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), interactionOpTimeout)
	defer cancel()
	res, workErr := economy.Work(ctx, userID, difficulty)
	if workErr != nil {
		var cdErr *service.CooldownError
		if errors.As(workErr, &cdErr) {
			embed := discord.NewEmbedBuilder().
				SetTitle("Workはクールダウン中です").
				SetDescription(fmt.Sprintf("次に実行できる時刻: <t:%d:R>", cdErr.Until.Unix())).
				SetColor(0xE67E22).
				Build()
			_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
				SetEmbeds(embed).
				SetContainerComponents(workDisabledRow(easyID, hardID)).
				Build())
			return
		}

		var issuanceErr *service.DailyIssuanceCapError
		if errors.As(workErr, &issuanceErr) {
			embed := discord.NewEmbedBuilder().
				SetTitle("本日の配布上限").
				SetDescription(fmt.Sprintf("本日の発行上限に達しました。上限: %d / 既発行: %d", issuanceErr.Cap, issuanceErr.Issued)).
				SetColor(0xE67E22).
				Build()
			_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
				SetEmbeds(embed).
				SetContainerComponents(workDisabledRow(easyID, hardID)).
				Build())
			return
		}

		var haltedErr *service.MarketHaltedError
		if errors.As(workErr, &haltedErr) {
			embed := discord.NewEmbedBuilder().
				SetTitle("市場停止中").
				SetDescription("市場は緊急停止中です。30分安定後に自動解除されます。").
				SetColor(0xE67E22).
				Build()
			_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
				SetEmbeds(embed).
				SetContainerComponents(workDisabledRow(easyID, hardID)).
				Build())
			return
		}

		logger.Error().Err(workErr).Msg("work button failed")
		embed := discord.NewEmbedBuilder().
			SetTitle("エラー").
			SetDescription("処理中にエラーが発生しました。時間をおいて再試行してください。").
			SetColor(0xE74C3C).
			Build()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(embed).
			SetContainerComponents(workDisabledRow(easyID, hardID)).
			Build())
		return
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Work 完了").
		SetColor(0x2ECC71).
		AddField("難易度", strings.ToUpper(string(res.Difficulty)), true).
		AddField("報酬", fmt.Sprintf("+%d %s", res.YenReward, service.CurrencyYenUnit), true).
		AddField("XP", fmt.Sprintf("+%d", res.XP), true).
		AddField("残りYen", fmt.Sprintf("%d %s", res.YenBalance, service.CurrencyYenUnit), true).
		AddField("保有ALToken", fmt.Sprintf("%d %s", res.AltBalance, service.CurrencyALTUnit), true).
		AddField("次回まで", fmt.Sprintf("<t:%d:R>", res.NextWorkAt.Unix()), true).
		SetTimestamp(time.Now()).
		Build()

	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(embed).
		SetContainerComponents(workDisabledRow(easyID, hardID)).
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

func workDisabledRow(easyID string, hardID string) discord.ContainerComponent {
	easy := discord.NewPrimaryButton("Easy", easyID)
	easy.Disabled = true
	hard := discord.NewDangerButton("Hard", hardID)
	hard.Disabled = true
	return discord.NewActionRow(easy, hard)
}

func buildWorkButtonID(userID string, difficulty service.WorkDifficulty, expiresAt int64) string {
	return fmt.Sprintf("work:%s:%s:%d", userID, difficulty, expiresAt)
}

func parseWorkButtonID(customID string) (string, service.WorkDifficulty, int64, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 4 || parts[0] != "work" {
		return "", "", 0, fmt.Errorf("invalid work custom id")
	}
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return "", "", 0, err
	}
	difficulty := service.WorkDifficulty(parts[2])
	if difficulty != service.WorkDifficultyEasy && difficulty != service.WorkDifficultyHard {
		return "", "", 0, fmt.Errorf("invalid work difficulty")
	}
	return parts[1], difficulty, expiresAt, nil
}
