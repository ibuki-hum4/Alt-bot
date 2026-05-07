package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alt-bot/ent"
	"alt-bot/ent/rolepanel"

	"github.com/disgoorg/snowflake/v2"
	"entgo.io/ent/dialect/sql"
)

type RolePanelService struct {
	client *ent.Client
}

type RolePanelSnapshot struct {
	ID          int
	GuildID     string
	ChannelID   string
	MessageID   string
	Title       string
	Description string
	RoleIDs     []string
	UpdatedAt   time.Time
}

func NewRolePanelService(client *ent.Client) *RolePanelService {
	return &RolePanelService{client: client}
}

func (s *RolePanelService) UpsertPanel(ctx context.Context, guildID, channelID, messageID snowflake.ID, title, description string, roleIDs []snowflake.ID) (*RolePanelSnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	roleIDsRaw := make([]string, 0, len(roleIDs))
	for _, id := range roleIDs {
		if id == 0 {
			continue
		}
		roleIDsRaw = append(roleIDsRaw, id.String())
	}
	joinedRoleIDs := strings.Join(roleIDsRaw, ",")

	panel, err := s.client.RolePanel.Query().
		Where(
			rolepanel.GuildIDEQ(guildID.String()),
			rolepanel.MessageIDEQ(messageID.String()),
		).
		Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("failed to query role panel: %w", err)
		}
		panel, err = s.client.RolePanel.Create().
			SetGuildID(guildID.String()).
			SetChannelID(channelID.String()).
			SetMessageID(messageID.String()).
			SetTitle(title).
			SetDescription(description).
			SetRoleIds(joinedRoleIDs).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create role panel record: %w", err)
		}
		result := snapshotFromEnt(panel)
		return &result, nil
	}

	updated, err := panel.Update().
		SetChannelID(channelID.String()).
		SetTitle(title).
		SetDescription(description).
		SetRoleIds(joinedRoleIDs).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update role panel record: %w", err)
	}
	result := snapshotFromEnt(updated)
	return &result, nil
}

func (s *RolePanelService) AppendRoles(ctx context.Context, guildID, messageID snowflake.ID, roleIDs []snowflake.ID) (*RolePanelSnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	panel, err := s.client.RolePanel.Query().
		Where(
			rolepanel.GuildIDEQ(guildID.String()),
			rolepanel.MessageIDEQ(messageID.String()),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load role panel record: %w", err)
	}

	existing := splitRoleIDsRaw(panel.RoleIds)
	seen := make(map[string]struct{}, len(existing)+len(roleIDs))
	for _, value := range existing {
		seen[value] = struct{}{}
	}
	for _, id := range roleIDs {
		if id == 0 {
			continue
		}
		value := id.String()
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		existing = append(existing, value)
	}

	updated, err := panel.Update().
		SetRoleIds(strings.Join(existing, ",")).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to append role panel roles: %w", err)
	}
	result := snapshotFromEnt(updated)
	return &result, nil
}

func (s *RolePanelService) ListPanels(ctx context.Context, guildID snowflake.ID, query string, limit int) ([]RolePanelSnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 25 {
		limit = 25
	}

	builder := s.client.RolePanel.Query().
		Where(rolepanel.GuildIDEQ(guildID.String()))
	if strings.TrimSpace(query) != "" {
		builder = builder.Where(rolepanel.TitleContainsFold(strings.TrimSpace(query)))
	}

	panels, err := builder.
		Order(rolepanel.ByUpdatedAt(sql.OrderDesc())).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list role panels: %w", err)
	}

	result := make([]RolePanelSnapshot, 0, len(panels))
	for _, panel := range panels {
		result = append(result, snapshotFromEnt(panel))
	}
	return result, nil
}

func (s *RolePanelService) GetPanelByID(ctx context.Context, guildID snowflake.ID, panelID int) (*RolePanelSnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	panel, err := s.client.RolePanel.Query().
		Where(
			rolepanel.GuildIDEQ(guildID.String()),
			rolepanel.IDEQ(panelID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load role panel record: %w", err)
	}
	result := snapshotFromEnt(panel)
	return &result, nil
}

func (s *RolePanelService) DeletePanelByID(ctx context.Context, guildID snowflake.ID, panelID int) (*RolePanelSnapshot, error) {
	ctx, cancel := withServiceTimeout(ctx)
	defer cancel()

	panel, err := s.client.RolePanel.Query().
		Where(
			rolepanel.GuildIDEQ(guildID.String()),
			rolepanel.IDEQ(panelID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load role panel record: %w", err)
	}

	result := snapshotFromEnt(panel)
	if err := s.client.RolePanel.DeleteOne(panel).Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete role panel record: %w", err)
	}
	return &result, nil
}

func snapshotFromEnt(panel *ent.RolePanel) RolePanelSnapshot {
	return RolePanelSnapshot{
		ID:          panel.ID,
		GuildID:     panel.GuildID,
		ChannelID:   panel.ChannelID,
		MessageID:   panel.MessageID,
		Title:       panel.Title,
		Description: panel.Description,
		RoleIDs:     splitRoleIDsRaw(panel.RoleIds),
		UpdatedAt:   panel.UpdatedAt,
	}
}

func splitRoleIDsRaw(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}