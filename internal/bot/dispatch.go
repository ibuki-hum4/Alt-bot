package bot

import (
	"strings"
	"time"

	rootcommands "alt-bot/internal/bot/commands"
	cmdcasino "alt-bot/internal/bot/commands/casino"
	cmdcrypto "alt-bot/internal/bot/commands/crypto"
	cmdmod "alt-bot/internal/bot/commands/mod"
	cmdutil "alt-bot/internal/bot/commands/util"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func (h *Handlers) dispatchApplicationCommand(event *events.ApplicationCommandInteractionCreate) {
	userID := event.User().ID.String()
	// Data.CommandName() works for every command type. SlashCommandInteractionData()
	// must not be used before the type is known: it is an unchecked type
	// assertion and panics on a context menu command.
	name := event.Data.CommandName()
	if ok, message := h.allowSlash(userID, name, time.Now()); !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	if event.Data.Type() == discord.ApplicationCommandTypeMessage {
		h.dispatchMessageCommand(event, name)
		return
	}

	switch name {
	case "ping":
		cmdutil.HandlePing(h.logger, event)
	case "help":
		cmdutil.HandleHelp(event)
	case "work":
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /work は利用できません。")
			return
		}
		cmdutil.HandleWorkSlash(h.logger, event)
	case "rob":
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /rob は利用できません。")
			return
		}
		cmdutil.HandleRob(h.logger, h.economy, event)
	case "shop":
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /shop は利用できません。")
			return
		}
		cmdutil.HandleShop(h.logger, h.economy, event)
	case "crypto":
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /crypto は利用できません。")
			return
		}
		if !h.cfg.CryptoEnabled {
			h.replyFeatureDisabledSlash(event, "Crypto", " /crypto は利用できません。")
			return
		}
		cmdcrypto.HandleCryptoSlash(h.logger, h.economy, event)
	case "casino":
		subName := ""
		if data := event.SlashCommandInteractionData(); data.SubCommandName != nil {
			subName = strings.ToLower(*data.SubCommandName)
		}
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /casino は利用できません。")
			return
		}
		if subName == "poker" {
			if !h.cfg.PokerEnabled {
				h.replyFeatureDisabledSlash(event, "Poker", " /casino poker は利用できません。")
				return
			}
		} else {
			if !h.cfg.CasinoEnabled {
				h.replyFeatureDisabledSlash(event, "Casino", " /casino は利用できません。")
				return
			}
		}
		cmdcasino.HandleCasino(h.economy, event)
	case "commands":
		cmdutil.HandleCommands(h.logger, event, h.ownerIDs)
	case "news":
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /news は利用できません。")
			return
		}
		cmdutil.HandleNews(h.logger, h.economy, h.SetNewsChannel, h.DisableNewsChannel, h.NewsChannelStatus, event)
	case "rate":
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /rate は利用できません。")
			return
		}
		cmdutil.HandleRate(event, h.economy)
	case "chart":
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledSlash(event, " /chart は利用できません。")
			return
		}
		select {
		case h.chartSem <- struct{}{}:
			defer func() { <-h.chartSem }()
		default:
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("現在チャート生成が混み合っています。数秒後に再試行してください。").
				SetEphemeral(true).
				Build())
			return
		}
		cmdutil.HandleChart(h.logger, h.economy, event)
	case "pin":
		if !h.cfg.StickyEnabled {
			h.replyFeatureDisabledSlash(event, "固定メッセージ", " /pin は利用できません。")
			return
		}
		cmdutil.HandleSticky(h.logger, h.DisableStickyMessage, h.StickyMessageStatus, event)
	case "rp":
		cmdutil.HandleRolePanel(h.logger, h.cfg, h.rolePanels, event)
	case "mod":
		if !h.cfg.ModEnabled {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("モデレーション機能は現在無効化されています。").
				SetEphemeral(true).
				Build())
			return
		}
		cmdmod.HandleModeration(event)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のコマンドです").
			SetEphemeral(true).
			Build())
	}
}

// dispatchMessageCommand routes the message context menu commands (right click
// a message, then Apps).
func (h *Handlers) dispatchMessageCommand(event *events.ApplicationCommandInteractionCreate, name string) {
	switch name {
	case rootcommands.StickyMessageCommandName:
		if !h.cfg.StickyEnabled {
			h.replyFeatureDisabledSlash(event, "固定メッセージ", " この操作は利用できません。")
			return
		}
		cmdutil.HandleStickyMessageCommand(h.logger, h.SetStickyMessage, event)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応の操作です").
			SetEphemeral(true).
			Build())
	}
}

func (h *Handlers) dispatchModalSubmit(event *events.ModalSubmitInteractionCreate) {
	switch {
	case strings.HasPrefix(event.Data.CustomID, cmdutil.StickyModalPrefix):
		if !h.cfg.StickyEnabled {
			_ = event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("固定メッセージ機能は現在無効化されています。").
				SetEphemeral(true).
				Build())
			return
		}
		cmdutil.HandleStickyModal(h.logger, h.SetStickyMessage, event)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応の入力フォームです").
			SetEphemeral(true).
			Build())
	}
}

func (h *Handlers) dispatchComponentInteraction(event *events.ComponentInteractionCreate) {
	if ok, message := h.allowComponent(event.User().ID.String(), time.Now()); !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	customID := event.Data.CustomID()
	switch {
	case strings.HasPrefix(customID, "work:"):
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledComponent(event)
			return
		}
		cmdutil.HandleWorkComponent(h.logger, h.economy, event)
	case strings.HasPrefix(customID, "shop:"):
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledComponent(event)
			return
		}
		cmdutil.HandleShopComponent(h.logger, h.economy, event)
	case strings.HasPrefix(customID, "crypto:"):
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledComponent(event)
			return
		}
		if !h.cfg.CryptoEnabled {
			h.replyFeatureDisabledComponent(event, "Crypto")
			return
		}
		cmdcrypto.HandleCryptoComponent(h.logger, h.economy, event)
	case strings.HasPrefix(customID, "casino:"):
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledComponent(event)
			return
		}
		if !h.cfg.CasinoEnabled {
			h.replyFeatureDisabledComponent(event, "Casino")
			return
		}
		cmdcasino.HandleCasinoComponent(h.economy, event)
	case strings.HasPrefix(customID, "blackjack:"):
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledComponent(event)
			return
		}
		if !h.cfg.CasinoEnabled {
			h.replyFeatureDisabledComponent(event, "Casino")
			return
		}
		cmdcasino.HandleBlackjackComponent(h.economy, event)
	case strings.HasPrefix(customID, "mines:"):
		if !h.cfg.EconomyEnabled {
			h.replyEconomyDisabledComponent(event)
			return
		}
		if !h.cfg.CasinoEnabled {
			h.replyFeatureDisabledComponent(event, "Casino")
			return
		}
		cmdcasino.HandleMinesComponent(h.economy, event)
	case strings.HasPrefix(customID, "rolepanel:"):
		cmdutil.HandleRolePanelComponent(h.logger, h.cfg, event)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のボタンです").
			SetEphemeral(true).
			Build())
	}
}
