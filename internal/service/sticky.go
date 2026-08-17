package service

import (
	"context"
	"fmt"
	"strings"

	"alt-bot/ent"
	"alt-bot/ent/stickymessage"

	"github.com/disgoorg/snowflake/v2"
)

// StickyMessageMaxLength caps the stored text well below Discord's 2000
// character message limit, leaving room for the command to reject oversized
// input up front instead of failing at post time.
const StickyMessageMaxLength = 1500

type StickyService struct {
	client *ent.Client
}

type StickySnapshot struct {
	GuildID       string
	ChannelID     string
	Content       string
	LastMessageID string
	Enabled       bool
	CreatedBy     string
}

func NewStickyService(client *ent.Client) *StickyService {
	return &StickyService{client: client}
}

func snapshotFromSticky(row *ent.StickyMessage) StickySnapshot {
	return StickySnapshot{
		GuildID:       row.GuildID,
		ChannelID:     row.ChannelID,
		Content:       row.Content,
		LastMessageID: row.LastMessageID,
		Enabled:       row.Enabled,
		CreatedBy:     row.CreatedBy,
	}
}

// Upsert sets or replaces the sticky message for a channel. Replacing the text
// keeps last_message_id so the caller can still delete the copy currently
// posted before putting up the new one.
func (s *StickyService) Upsert(ctx context.Context, guildID, channelID snowflake.ID, content, createdBy string) (*StickySnapshot, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, fmt.Errorf("sticky content must not be empty")
	}
	if len(trimmed) > StickyMessageMaxLength {
		return nil, fmt.Errorf("sticky content exceeds %d characters", StickyMessageMaxLength)
	}

	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	row, err := s.client.StickyMessage.Query().
		Where(stickymessage.ChannelIDEQ(channelID.String())).
		Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("failed to query sticky message: %w", err)
		}
		created, createErr := s.client.StickyMessage.Create().
			SetGuildID(guildID.String()).
			SetChannelID(channelID.String()).
			SetContent(trimmed).
			SetEnabled(true).
			SetCreatedBy(createdBy).
			Save(ctx)
		if createErr != nil {
			return nil, fmt.Errorf("failed to create sticky message: %w", createErr)
		}
		result := snapshotFromSticky(created)
		return &result, nil
	}

	updated, err := row.Update().
		SetGuildID(guildID.String()).
		SetContent(trimmed).
		SetEnabled(true).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update sticky message: %w", err)
	}
	result := snapshotFromSticky(updated)
	return &result, nil
}

// Disable removes the sticky for a channel and returns what was stored, so the
// caller can delete the copy still sitting in the channel.
func (s *StickyService) Disable(ctx context.Context, channelID snowflake.ID) (*StickySnapshot, bool, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	row, err := s.client.StickyMessage.Query().
		Where(stickymessage.ChannelIDEQ(channelID.String())).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to load sticky message: %w", err)
	}

	result := snapshotFromSticky(row)
	if err := s.client.StickyMessage.DeleteOne(row).Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("failed to delete sticky message: %w", err)
	}
	return &result, true, nil
}

func (s *StickyService) Get(ctx context.Context, channelID snowflake.ID) (*StickySnapshot, bool, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	row, err := s.client.StickyMessage.Query().
		Where(stickymessage.ChannelIDEQ(channelID.String())).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to load sticky message: %w", err)
	}
	result := snapshotFromSticky(row)
	return &result, true, nil
}

// ListEnabled returns every active sticky, used at startup to build the
// in-memory channel set so ordinary messages never hit the database.
func (s *StickyService) ListEnabled(ctx context.Context) ([]StickySnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	rows, err := s.client.StickyMessage.Query().
		Where(stickymessage.EnabledEQ(true)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sticky messages: %w", err)
	}

	result := make([]StickySnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, snapshotFromSticky(row))
	}
	return result, nil
}

// ListByGuild backs the per-guild view in the web dashboard.
func (s *StickyService) ListByGuild(ctx context.Context, guildID snowflake.ID) ([]StickySnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	rows, err := s.client.StickyMessage.Query().
		Where(stickymessage.GuildIDEQ(guildID.String())).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sticky messages for guild: %w", err)
	}

	result := make([]StickySnapshot, 0, len(rows))
	for _, row := range rows {
		result = append(result, snapshotFromSticky(row))
	}
	return result, nil
}

// SetLastMessageID records which copy is currently posted in the channel.
// stillConfigured is false when the sticky was removed in the meantime, which
// tells the caller the message it just posted is now orphaned.
func (s *StickyService) SetLastMessageID(ctx context.Context, channelID snowflake.ID, messageID string) (stillConfigured bool, err error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	updated, err := s.client.StickyMessage.Update().
		Where(stickymessage.ChannelIDEQ(channelID.String())).
		SetLastMessageID(messageID).
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to update sticky last message id: %w", err)
	}
	return updated > 0, nil
}
