package util

import (
	"errors"
	"fmt"

	rootcommands "alt-bot/internal/bot/commands"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/rs/zerolog"
)

func HandleCommands(logger zerolog.Logger, event *events.ApplicationCommandInteractionCreate, ownerIDs map[string]struct{}) {
	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil || *data.SubCommandName != "reload" {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。").
			SetEphemeral(true).
			Build())
		return
	}

	if len(ownerIDs) == 0 {
		logger.Warn().Msg("owner id list is empty; deny /commands reload")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("OWNER_IDが未設定です。管理者に連絡してください。").
			SetEphemeral(true).
			Build())
		return
	}

	if _, ok := ownerIDs[event.User().ID.String()]; !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("この操作はBot Ownerのみ実行できます。").
			SetEphemeral(true).
			Build())
		return
	}

	_, err := event.Client().Rest().SetGlobalCommands(event.Client().ApplicationID(), rootcommands.Definitions())
	if err != nil {
		var restErr rest.Error
		if errors.As(err, &restErr) {
			logger.Error().Err(err).Any("rest_error", restErr).Msg("failed to reload commands")
		} else {
			logger.Error().Err(err).Msg("failed to reload commands")
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(fmt.Sprintf("コマンド再登録に失敗しました: %v", err)).
			SetEphemeral(true).
			Build())
		return
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent("コマンドを再登録しました。").
		SetEphemeral(true).
		Build())
}
