package bot

import (
	"context"
	"time"

	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

const (
	defaultStickyDebounce = 5 * time.Second
	stickyOpTimeout       = 5 * time.Second
)

// stickyDebounceDuration resolves the configured debounce window, clamped so a
// misconfigured value cannot turn every message into a repost (which would run
// straight into Discord's rate limits).
func stickyDebounceDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultStickyDebounce
	}
	if seconds > 300 {
		return 300 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// StartSticky captures the client used for reposting and warms the channel
// cache so ordinary messages never need to query the database.
func (h *Handlers) StartSticky(ctx context.Context, client bot.Client) {
	if !h.cfg.StickyEnabled {
		return
	}

	h.stickyMu.Lock()
	h.stickyClient = client
	h.stickyMu.Unlock()

	stickies, err := h.sticky.ListEnabled(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to load sticky messages; they will not repost until reconfigured")
		return
	}

	h.stickyMu.Lock()
	for _, s := range stickies {
		channelID, parseErr := snowflake.Parse(s.ChannelID)
		if parseErr != nil {
			h.logger.Warn().Str("channel_id", s.ChannelID).Msg("skipping sticky with unparsable channel id")
			continue
		}
		h.stickyChannels[channelID] = struct{}{}
	}
	count := len(h.stickyChannels)
	h.stickyMu.Unlock()

	h.logger.Info().Int("channels", count).Msg("sticky messages loaded")
}

// StopStickyTimers cancels pending reposts so a shutdown does not leave timers
// firing against a closing client.
func (h *Handlers) StopStickyTimers() {
	h.stickyMu.Lock()
	defer h.stickyMu.Unlock()
	for channelID, timer := range h.stickyTimers {
		timer.Stop()
		delete(h.stickyTimers, channelID)
	}
}

// OnMessageCreate schedules a sticky repost for the channel the message landed
// in. Reposting is debounced, so a burst of chatter collapses into a single
// repost once the channel goes quiet.
func (h *Handlers) OnMessageCreate(event *events.MessageCreate) {
	if !h.cfg.StickyEnabled {
		return
	}
	// Sticky messages are a guild feature, and skipping bots is what keeps the
	// bot's own repost from triggering another repost forever.
	if event.GuildID == nil || event.Message.Author.Bot {
		return
	}

	channelID := event.ChannelID
	client := event.Client()

	h.stickyMu.Lock()
	defer h.stickyMu.Unlock()

	if _, ok := h.stickyChannels[channelID]; !ok {
		return
	}
	if timer, ok := h.stickyTimers[channelID]; ok {
		timer.Stop()
	}
	h.stickyTimers[channelID] = time.AfterFunc(h.stickyDebounce, func() {
		h.repostSticky(client, channelID)
	})
}

// repostSticky deletes the copy currently posted in the channel and posts the
// sticky again so it sits at the bottom.
func (h *Handlers) repostSticky(client bot.Client, channelID snowflake.ID) {
	h.stickyMu.Lock()
	delete(h.stickyTimers, channelID)
	if h.stickyReposting[channelID] {
		// A repost is already running for this channel. Letting a second one
		// through would post the sticky twice.
		h.stickyMu.Unlock()
		return
	}
	h.stickyReposting[channelID] = true
	h.stickyMu.Unlock()

	defer func() {
		h.stickyMu.Lock()
		delete(h.stickyReposting, channelID)
		h.stickyMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), stickyOpTimeout)
	defer cancel()

	snapshot, found, err := h.sticky.Get(ctx, channelID)
	if err != nil {
		h.logger.Error().Err(err).Str("channel_id", channelID.String()).Msg("failed to load sticky message for repost")
		return
	}
	if !found || !snapshot.Enabled {
		h.forgetStickyChannel(channelID)
		return
	}

	if snapshot.LastMessageID != "" {
		if previousID, parseErr := snowflake.Parse(snapshot.LastMessageID); parseErr == nil {
			// A failure here is expected when someone deleted the message by
			// hand, so it must not stop the repost.
			if delErr := client.Rest().DeleteMessage(channelID, previousID); delErr != nil {
				h.logger.Debug().Err(delErr).
					Str("channel_id", channelID.String()).
					Str("message_id", snapshot.LastMessageID).
					Msg("could not delete previous sticky message")
			}
		}
	}

	posted, err := client.Rest().CreateMessage(channelID, discord.NewMessageCreateBuilder().
		SetContent(snapshot.Content).
		Build())
	if err != nil {
		h.logger.Error().Err(err).Str("channel_id", channelID.String()).Msg("failed to post sticky message")
		return
	}

	stillConfigured, err := h.sticky.SetLastMessageID(ctx, channelID, posted.ID.String())
	if err != nil {
		h.logger.Error().Err(err).Str("channel_id", channelID.String()).Msg("failed to record sticky message id")
		return
	}
	if !stillConfigured {
		// /pin off ran while this repost was in flight, so nothing owns the
		// message that was just posted. Clean it up instead of leaving it.
		if delErr := client.Rest().DeleteMessage(channelID, posted.ID); delErr != nil {
			h.logger.Warn().Err(delErr).
				Str("channel_id", channelID.String()).
				Msg("could not remove sticky message posted during disable")
		}
		h.forgetStickyChannel(channelID)
	}
}

func (h *Handlers) forgetStickyChannel(channelID snowflake.ID) {
	h.stickyMu.Lock()
	defer h.stickyMu.Unlock()
	delete(h.stickyChannels, channelID)
	if timer, ok := h.stickyTimers[channelID]; ok {
		timer.Stop()
		delete(h.stickyTimers, channelID)
	}
}

// SetStickyMessage backs /pin set.
func (h *Handlers) SetStickyMessage(guildID, channelID snowflake.ID, content, createdBy string) error {
	ctx, cancel := context.WithTimeout(context.Background(), stickyOpTimeout)
	defer cancel()

	if _, err := h.sticky.Upsert(ctx, guildID, channelID, content, createdBy); err != nil {
		return err
	}

	h.stickyMu.Lock()
	h.stickyChannels[channelID] = struct{}{}
	h.stickyMu.Unlock()
	return nil
}

// DisableStickyMessage backs /pin off, removing the stored sticky and the copy
// still posted in the channel.
func (h *Handlers) DisableStickyMessage(channelID snowflake.ID) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stickyOpTimeout)
	defer cancel()

	snapshot, existed, err := h.sticky.Disable(ctx, channelID)
	if err != nil {
		return false, err
	}

	h.forgetStickyChannel(channelID)
	if !existed {
		return false, nil
	}

	h.stickyMu.Lock()
	client := h.stickyClient
	h.stickyMu.Unlock()

	if client != nil && snapshot.LastMessageID != "" {
		if previousID, parseErr := snowflake.Parse(snapshot.LastMessageID); parseErr == nil {
			if delErr := client.Rest().DeleteMessage(channelID, previousID); delErr != nil {
				h.logger.Debug().Err(delErr).
					Str("channel_id", channelID.String()).
					Msg("could not delete sticky message while disabling")
			}
		}
	}
	return true, nil
}

// StickyMessageStatus backs /pin status.
func (h *Handlers) StickyMessageStatus(channelID snowflake.ID) (*service.StickySnapshot, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stickyOpTimeout)
	defer cancel()
	return h.sticky.Get(ctx, channelID)
}
