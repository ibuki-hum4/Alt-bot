package util

import (
	"fmt"
	"strings"

	"alt-bot/internal/bot/commands/guildperm"
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rs/zerolog"
)

// HandleSticky implements /pin set|off|status. The callbacks are supplied by
// the bot package so it can keep its in-memory channel cache in step with the
// database, matching how HandleNews is wired.
func HandleSticky(
	logger zerolog.Logger,
	setSticky func(guildID snowflake.ID, channelID snowflake.ID, content string, createdBy string) error,
	disableSticky func(channelID snowflake.ID) (bool, error),
	statusSticky func(channelID snowflake.ID) (*service.StickySnapshot, bool, error),
	event *events.ApplicationCommandInteractionCreate,
) {
	guildID, permMessage, ok := guildperm.CheckManageGuild(event)
	if !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(permMessage).
			SetEphemeral(true).
			Build())
		return
	}

	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/pin set|off|status を指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	channelID := event.ChannelID()

	switch strings.ToLower(*data.SubCommandName) {
	case "set":
		content := strings.TrimSpace(data.String("message"))
		if content == "" {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("message を指定してください。").
				SetEphemeral(true).
				Build())
			return
		}
		if len(content) > service.StickyMessageMaxLength {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent(fmt.Sprintf("message が長すぎます。%d 文字以内で指定してください。", service.StickyMessageMaxLength)).
				SetEphemeral(true).
				Build())
			return
		}

		if err := setSticky(guildID, channelID, content, event.User().ID.String()); err != nil {
			logger.Error().Err(err).
				Str("guild_id", guildID.String()).
				Str("channel_id", channelID.String()).
				Msg("failed to configure sticky message")
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("固定メッセージの設定に失敗しました。時間をおいて再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		logger.Info().
			Str("guild_id", guildID.String()).
			Str("channel_id", channelID.String()).
			Msg("sticky message configured")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このチャンネルの固定メッセージを設定しました。次の発言から最下部に再投稿されます。\n解除するには `/pin off` を実行してください。").
			SetEphemeral(true).
			Build())

	case "off":
		existed, err := disableSticky(channelID)
		if err != nil {
			logger.Error().Err(err).
				Str("channel_id", channelID.String()).
				Msg("failed to disable sticky message")
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("固定メッセージの解除に失敗しました。時間をおいて再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		if !existed {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("このチャンネルには固定メッセージが設定されていません。").
				SetEphemeral(true).
				Build())
			return
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このチャンネルの固定メッセージを解除しました。").
			SetEphemeral(true).
			Build())

	case "status":
		snapshot, found, err := statusSticky(channelID)
		if err != nil {
			logger.Error().Err(err).
				Str("channel_id", channelID.String()).
				Msg("failed to load sticky message status")
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("固定メッセージの取得に失敗しました。時間をおいて再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		if !found {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("このチャンネルには固定メッセージが設定されていません。").
				SetEphemeral(true).
				Build())
			return
		}

		state := "有効"
		if !snapshot.Enabled {
			state = "停止中"
		}
		embed := discord.NewEmbedBuilder().
			SetTitle("固定メッセージ設定").
			SetColor(0x5865F2).
			AddField("状態", state, true).
			AddField("設定者", fmt.Sprintf("<@%s>", snapshot.CreatedBy), true).
			AddField("本文", snapshot.Content, false).
			Build()
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEmbeds(embed).
			SetEphemeral(true).
			Build())

	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/pin set|off|status を指定してください。").
			SetEphemeral(true).
			Build())
	}
}
