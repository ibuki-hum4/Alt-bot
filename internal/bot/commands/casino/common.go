package casino

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const casinoInteractionTimeout = 90 * time.Second

type casinoSession struct {
	Game     string
	UserID   string
	Bet      int64
	Expires  int64
	Option   string
	GuildID  string
	Title    string
	Subtitle string
}

func startCasinoSession(
	event *events.ApplicationCommandInteractionCreate,
	game string,
	title string,
	description string,
	guildIDText string,
	option string,
) {
	bet := int64(event.SlashCommandInteractionData().Int("amount"))
	if bet <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("amount は 1 以上で指定してください。例: /casino <game> amount:100").
			SetEphemeral(true).
			Build())
		return
	}

	expiresAt := time.Now().Add(casinoInteractionTimeout).Unix()
	userID := event.User().ID.String()
	playID := buildCasinoPlayID(game, userID, bet, expiresAt, option, guildIDText)
	cancelID := buildCasinoCancelID(game, userID, bet, expiresAt, option, guildIDText)

	preset := discord.NewEmbedBuilder().
		SetTitle(title+" 準備完了").
		SetDescription(description).
		SetColor(0x3498DB).
		AddField("Bet", fmt.Sprintf("%d %s", bet, service.CurrencyYenUnit), true).
		AddField("操作", "下のボタンでプレイ", true).
		AddField("有効期限", fmt.Sprintf("<t:%d:R>", expiresAt), true).
		AddField("Guild", guildIDText, true)
	if strings.TrimSpace(option) != "" {
		preset.AddField("Option", option, true)
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEmbeds(preset.Build()).
		AddActionRow(
			discord.NewSuccessButton("プレイ", playID),
			discord.NewSecondaryButton("終了", cancelID),
		).
		SetEphemeral(true).
		Build())
}

func HandleCasinoComponent(economy *service.EconomyService, event *events.ComponentInteractionCreate) {
	s, action, err := parseCasinoComponentID(event.Data.CustomID())
	if err != nil {
		return
	}

	if event.User().ID.String() != s.UserID {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このボタンはコマンド実行者のみ操作できます。").
			SetEphemeral(true).
			Build())
		return
	}

	if time.Now().Unix() > s.Expires {
		embed := discord.NewEmbedBuilder().
			SetTitle("タイムアウト").
			SetDescription("操作の有効期限が切れました。再度 /casino を実行してください。").
			SetColor(0x95A5A6).
			Build()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(embed).
			ClearContainerComponents().
			Build())
		return
	}

	if action == "cancel" {
		embed := discord.NewEmbedBuilder().
			SetTitle(s.Title + " 終了").
			SetDescription("セッションを終了しました。").
			SetColor(0x95A5A6).
			Build()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(embed).
			ClearContainerComponents().
			Build())
		return
	}

	if s.Game == "mines" {
		embed := discord.NewEmbedBuilder().
			SetTitle("Mines 移行").
			SetDescription("Mines は GUI 版に移行しました。/casino mines から開始してください。").
			SetColor(0x95A5A6).
			Build()
		_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
			SetEmbeds(embed).
			ClearContainerComponents().
			Build())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, playErr := executeCasinoGame(ctx, economy, s)
	if playErr != nil {
		var insufficient *service.InsufficientYenError
		if errors.As(playErr, &insufficient) {
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

	resultTitle := s.Title + " 結果"
	color := 0x95A5A6
	if res.NetYen > 0 {
		resultTitle = s.Title + " 勝利"
		color = 0x2ECC71
	} else if res.NetYen < 0 {
		resultTitle = s.Title + " 残念"
		color = 0xE74C3C
	}

	playID := buildCasinoPlayID(s.Game, s.UserID, s.Bet, s.Expires, s.Option, s.GuildID)
	cancelID := buildCasinoCancelID(s.Game, s.UserID, s.Bet, s.Expires, s.Option, s.GuildID)

	_ = event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitle(resultTitle).
			SetDescription(s.Subtitle).
			SetColor(color).
			AddField("Result", strings.Join(res.Symbols, " | "), false).
			AddField("倍率", fmt.Sprintf("%.2fx", res.Multiplier), true).
			AddField("Bet", fmt.Sprintf("%d %s", res.BetYen, service.CurrencyYenUnit), true).
			AddField("払戻", fmt.Sprintf("%d %s", res.PayoutYen, service.CurrencyYenUnit), true).
			AddField("収支", fmt.Sprintf("%+d %s", res.NetYen, service.CurrencyYenUnit), true).
			AddField("残りYen", fmt.Sprintf("%d %s", res.YenBalance, service.CurrencyYenUnit), true).
			AddField("Guild", s.GuildID, true).
			SetTimestamp(time.Now()).
			Build()).
		SetContainerComponents(discord.NewActionRow(
			discord.NewSuccessButton("もう一回", playID),
			discord.NewSecondaryButton("終了", cancelID),
		)).
		Build())

	switch s.Game {
	case "blackjack", "chinchiro":
		if pngBytes, renderErr := renderCasinoResultPNG(s.Game, res); renderErr == nil {
			fileName := fmt.Sprintf("%s-result.png", s.Game)
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent(fmt.Sprintf("%s の画像結果です。", s.Title)).
				AddFile(fileName, s.Title+" result", bytes.NewReader(pngBytes)).
				SetEphemeral(true).
				Build())
		}
	}
}

func executeCasinoGame(ctx context.Context, economy *service.EconomyService, s casinoSession) (service.CasinoPlayResult, error) {
	switch s.Game {
	case "blackjack":
		return economy.PlayBlackjack(ctx, s.UserID, s.Bet)
	case "chinchiro":
		return economy.PlayChinchiro(ctx, s.UserID, s.Bet)
	default:
		return service.CasinoPlayResult{}, fmt.Errorf("unknown casino game")
	}
}

func buildCasinoPlayID(game string, userID string, bet int64, expires int64, option string, guildID string) string {
	opt := option
	if strings.TrimSpace(opt) == "" {
		opt = "_"
	}
	return fmt.Sprintf("casino:play:%s:%s:%d:%d:%s:%s", game, userID, bet, expires, opt, guildID)
}

func buildCasinoCancelID(game string, userID string, bet int64, expires int64, option string, guildID string) string {
	opt := option
	if strings.TrimSpace(opt) == "" {
		opt = "_"
	}
	return fmt.Sprintf("casino:cancel:%s:%s:%d:%d:%s:%s", game, userID, bet, expires, opt, guildID)
}

func parseCasinoComponentID(customID string) (casinoSession, string, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 8 || parts[0] != "casino" {
		return casinoSession{}, "", fmt.Errorf("invalid casino custom id")
	}
	action := parts[1]
	if action != "play" && action != "cancel" {
		return casinoSession{}, "", fmt.Errorf("invalid casino action")
	}
	bet, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return casinoSession{}, "", err
	}
	expires, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil {
		return casinoSession{}, "", err
	}
	option := parts[6]
	if option == "_" {
		option = ""
	}
	s := casinoSession{
		Game:    parts[2],
		UserID:  parts[3],
		Bet:     bet,
		Expires: expires,
		Option:  option,
		GuildID: parts[7],
	}
	s.Title, s.Subtitle = casinoLabels(s.Game, option)
	return s, action, nil
}

func casinoLabels(game string, option string) (string, string) {
	switch game {
	case "blackjack":
		return "Blackjack", "手札を見比べよう。"
	case "chinchiro":
		return "Chinchiro", "出目を見比べよう。"
	case "mines":
		return "Mines", "安全マスを引けるか挑戦。"
	default:
		return "Casino", "カジノセッション"
	}
}
