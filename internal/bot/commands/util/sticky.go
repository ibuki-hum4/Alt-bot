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

const (
	// StickyModalPrefix identifies the /pin set modal so the bot can route its
	// submission back here.
	StickyModalPrefix = "sticky:set:"

	stickyModalInputID = "content"

	stickyEmbedColor = 0x5865F2
)

// BuildStickyEmbed renders the sticky as it appears in the channel. It is
// exported so the repost loop and the /pin confirmation show the same thing.
func BuildStickyEmbed(content string) discord.Embed {
	return discord.NewEmbedBuilder().
		SetTitle("📌 固定メッセージ").
		SetDescription(content).
		SetColor(stickyEmbedColor).
		Build()
}

// StickySetter stores the sticky text for a channel.
type StickySetter func(guildID snowflake.ID, channelID snowflake.ID, content string, createdBy string) error

// HandleSticky implements /pin set|off|status. The callbacks are supplied by
// the bot package so it can keep its in-memory channel cache in step with the
// database, matching how HandleNews is wired.
func HandleSticky(
	logger zerolog.Logger,
	disableSticky func(channelID snowflake.ID) (bool, error),
	statusSticky func(channelID snowflake.ID) (*service.StickySnapshot, bool, error),
	event *events.ApplicationCommandInteractionCreate,
) {
	if _, permMessage, ok := guildperm.CheckManageGuild(event); !ok {
		replyStickyEphemeral(event, permMessage)
		return
	}

	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		replyStickyEphemeral(event, "/pin set|off|status を指定してください。")
		return
	}

	channelID := event.ChannelID()

	switch strings.ToLower(*data.SubCommandName) {
	case "set":
		// The text is collected in a modal so it can be multi-line and long,
		// which a slash command option cannot comfortably do.
		input := discord.NewParagraphTextInput(stickyModalInputID, "固定表示する本文").
			WithRequired(true).
			WithMaxLength(service.StickyMessageMaxLength).
			WithPlaceholder("このチャンネルに常に表示しておきたい内容")

		existing, found, err := statusSticky(channelID)
		if err == nil && found {
			// Pre-fill so editing an existing sticky does not mean retyping it.
			input = input.WithValue(existing.Content)
		}

		if err := event.Modal(discord.NewModalCreateBuilder().
			SetCustomID(StickyModalPrefix + channelID.String()).
			SetTitle("固定メッセージの設定").
			AddActionRow(input).
			Build()); err != nil {
			logger.Error().Err(err).Str("channel_id", channelID.String()).Msg("failed to open sticky modal")
		}

	case "off":
		existed, err := disableSticky(channelID)
		if err != nil {
			logger.Error().Err(err).
				Str("channel_id", channelID.String()).
				Msg("failed to disable sticky message")
			replyStickyEphemeral(event, "固定メッセージの解除に失敗しました。時間をおいて再試行してください。")
			return
		}
		if !existed {
			replyStickyEphemeral(event, "このチャンネルには固定メッセージが設定されていません。")
			return
		}
		replyStickyEphemeral(event, "このチャンネルの固定メッセージを解除しました。")

	case "status":
		snapshot, found, err := statusSticky(channelID)
		if err != nil {
			logger.Error().Err(err).
				Str("channel_id", channelID.String()).
				Msg("failed to load sticky message status")
			replyStickyEphemeral(event, "固定メッセージの取得に失敗しました。時間をおいて再試行してください。")
			return
		}
		if !found {
			replyStickyEphemeral(event, "このチャンネルには固定メッセージが設定されていません。")
			return
		}

		state := "有効"
		if !snapshot.Enabled {
			state = "停止中"
		}
		embed := discord.NewEmbedBuilder().
			SetTitle("固定メッセージ設定").
			SetColor(stickyEmbedColor).
			AddField("状態", state, true).
			AddField("設定者", fmt.Sprintf("<@%s>", snapshot.CreatedBy), true).
			AddField("本文", snapshot.Content, false).
			Build()
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetEmbeds(embed).
			SetEphemeral(true).
			Build())

	default:
		replyStickyEphemeral(event, "/pin set|off|status を指定してください。")
	}
}

// HandleStickyModal stores the text submitted through the /pin set modal.
func HandleStickyModal(
	logger zerolog.Logger,
	setSticky StickySetter,
	event *events.ModalSubmitInteractionCreate,
) {
	// The permission is re-checked here rather than trusting the check that ran
	// when the modal was opened, since the member's roles may have changed.
	guildID, permMessage, ok := guildperm.CheckManageGuild(event)
	if !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(permMessage).
			SetEphemeral(true).
			Build())
		return
	}

	rawChannelID := strings.TrimPrefix(event.Data.CustomID, StickyModalPrefix)
	channelID, err := snowflake.Parse(rawChannelID)
	if err != nil {
		logger.Warn().Str("custom_id", event.Data.CustomID).Msg("unparsable sticky modal custom id")
		return
	}

	content := strings.TrimSpace(event.Data.Text(stickyModalInputID))
	if content == "" {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("本文が空です。もう一度 /pin set を実行してください。").
			SetEphemeral(true).
			Build())
		return
	}
	if len(content) > service.StickyMessageMaxLength {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("本文が長すぎます。%d 文字以内で指定してください。", service.StickyMessageMaxLength)).
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
		SetEmbeds(discord.NewEmbedBuilder().
			SetTitle("固定メッセージを設定しました").
			SetDescription(content).
			SetColor(stickyEmbedColor).
			SetFooterText("次の発言から最下部に再投稿されます。解除は /pin off").
			Build()).
		SetEphemeral(true).
		Build())
}

func replyStickyEphemeral(event *events.ApplicationCommandInteractionCreate, content string) {
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(content).
		SetEphemeral(true).
		Build())
}
