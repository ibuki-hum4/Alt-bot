package util

import (
	"fmt"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rs/zerolog"
)

func HandleNews(
	logger zerolog.Logger,
	economy *service.EconomyService,
	setChannel func(guildID snowflake.ID, channelID snowflake.ID) error,
	disableChannel func(guildID snowflake.ID) error,
	statusChannel func(guildID snowflake.ID) (snowflake.ID, bool, error),
	event *events.ApplicationCommandInteractionCreate,
) {
	data := event.SlashCommandInteractionData()

	if event.GuildID() == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/news はサーバー内でのみ設定できます。").
			SetEphemeral(true).
			Build())
		return
	}

	if data.SubCommandName == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/news channel|off|status を指定してください。").
			SetEphemeral(true).
			Build())
		return
	}

	guildID := *event.GuildID()
	switch *data.SubCommandName {
	case "channel":
		channel, ok := data.OptChannel("channel")
		if !ok {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("channel オプションの取得に失敗しました。").
				SetEphemeral(true).
				Build())
			return
		}

		if err := setChannel(guildID, channel.ID); err != nil {
			logger.Error().Err(err).Str("guild_id", guildID.String()).Str("channel_id", channel.ID.String()).Msg("failed to configure news auto channel")
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("ニュース自動配信チャンネルの設定に失敗しました。時間をおいて再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		logger.Info().Str("guild_id", guildID.String()).Str("channel_id", channel.ID.String()).Msg("news auto channel configured")
		prob := economy.CurrentNewsProbability(time.Now())

		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("ニュース自動配信チャンネルを <#%s> に設定しました。現在当選率: %.3f%%", channel.ID.String(), prob.Final*100)).
			SetEphemeral(true).
			Build())

	case "off":
		if err := disableChannel(guildID); err != nil {
			logger.Error().Err(err).Str("guild_id", guildID.String()).Msg("failed to disable news auto channel")
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("ニュース自動配信の停止に失敗しました。時間をおいて再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ニュース自動配信を停止しました。").
			SetEphemeral(true).
			Build())

	case "status":
		channelID, ok, err := statusChannel(guildID)
		if err != nil {
			logger.Error().Err(err).Str("guild_id", guildID.String()).Msg("failed to get news status")
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("ニュース設定の取得に失敗しました。時間をおいて再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		if !ok {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("ニュース自動配信は未設定です。/news channel で設定してください。").
				SetEphemeral(true).
				Build())
			return
		}
		prob := economy.CurrentNewsProbability(time.Now())
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("現在のニュース配信先は <#%s> です。現在当選率: %.3f%%", channelID.String(), prob.Final*100)).
			SetEphemeral(true).
			Build())

	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("/news channel|off|status を指定してください。").
			SetEphemeral(true).
			Build())
	}
}
