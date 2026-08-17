package util

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"alt-bot/internal/bot/commands/guildperm"
	"alt-bot/internal/bot/rolepanel"
	"alt-bot/internal/config"
	"alt-bot/internal/service"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
	"github.com/rs/zerolog"
)

const (
	rolePanelCustomIDPrefix   = "rolepanel:"
	rolePanelMaxOptions       = 25
	rolePanelMaxRoles         = rolePanelMaxOptions - 1
	rolePanelPlaceholderValue = "__placeholder__"
	rolePanelDefaultTitle     = "役職パネル"
	rolePanelDefaultDesc      = "このパネルから役職を取得できます。"
	rolePanelDefaultPlacehold = "ロールを選択"
)

func HandleRolePanel(logger zerolog.Logger, cfg config.Config, rolePanels *service.RolePanelService, event *events.ApplicationCommandInteractionCreate) {
	guildID, message, ok := guildperm.CheckManageGuild(event)
	if !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	if ok, message := allowRolePanel(cfg, guildID); !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	data := event.SlashCommandInteractionData()
	if data.SubCommandName == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("サブコマンドを指定してください。 (create/add/delete)").
			SetEphemeral(true).
			Build())
		return
	}

	switch *data.SubCommandName {
	case "create":
		handleRolePanelCreate(logger, rolePanels, event)
	case "add":
		handleRolePanelAdd(logger, rolePanels, event)
	case "delete":
		handleRolePanelDelete(logger, rolePanels, event)
	default:
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("未対応のサブコマンドです。 (create/add/delete)").
			SetEphemeral(true).
			Build())
	}
}

func HandleRolePanelAutocomplete(logger zerolog.Logger, cfg config.Config, rolePanels *service.RolePanelService, event *events.AutocompleteInteractionCreate) {
	guildID := event.GuildID()
	if guildID == nil {
		return
	}

	if ok, _ := allowRolePanel(cfg, *guildID); !ok {
		return
	}

	data := event.Data
	if data.CommandName != "rp" {
		return
	}
	focused := data.Focused()
	if focused.Name != "panel" {
		return
	}

	query := strings.TrimSpace(data.String("panel"))
	panels, err := rolePanels.ListPanels(context.Background(), *guildID, query, 25)
	if err != nil {
		logger.Warn().Err(err).Str("guild_id", guildID.String()).Msg("failed to list role panels for autocomplete")
		_ = event.AutocompleteResult(nil)
		return
	}

	choices := make([]discord.AutocompleteChoice, 0, len(panels))
	for _, panel := range panels {
		name := truncateRolePanelText(panel.Title, 100)
		if name == "" {
			name = fmt.Sprintf("ロールパネル #%d", panel.ID)
		}
		choices = append(choices, discord.AutocompleteChoiceString{Name: name, Value: strconv.Itoa(panel.ID)})
	}
	_ = event.AutocompleteResult(choices)
}

func HandleRolePanelComponent(logger zerolog.Logger, cfg config.Config, event *events.ComponentInteractionCreate) {
	data, ok := event.Data.(discord.StringSelectMenuInteractionData)
	if !ok {
		return
	}
	if !strings.HasPrefix(data.CustomID(), rolePanelCustomIDPrefix) {
		return
	}

	guildID := event.GuildID()
	if guildID == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("この操作はサーバー内でのみ使用できます。").
			SetEphemeral(true).
			Build())
		return
	}

	if ok, message := allowRolePanel(cfg, *guildID); !ok {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	messageIDRaw := strings.TrimPrefix(data.CustomID(), rolePanelCustomIDPrefix)
	messageID, err := snowflake.Parse(messageIDRaw)
	if err != nil || messageID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルの識別子が不正です。").
			SetEphemeral(true).
			Build())
		return
	}

	selected := filterRolePanelValues(data.Values)
	if len(selected) == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールを選択してください。").
			SetEphemeral(true).
			Build())
		return
	}

	member, err := event.Client().Rest().GetMember(*guildID, event.User().ID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch member for role panel")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("メンバー情報の取得に失敗しました。もう一度お試しください。").
			SetEphemeral(true).
			Build())
		return
	}

	added := make([]string, 0, len(selected))
	removed := make([]string, 0, len(selected))
	failed := make([]string, 0, len(selected))

	for _, value := range selected {
		roleID, parseErr := snowflake.Parse(value)
		if parseErr != nil || roleID == 0 {
			failed = append(failed, value)
			continue
		}

		if memberHasRole(member, roleID) {
			if err := event.Client().Rest().RemoveMemberRole(*guildID, event.User().ID, roleID); err != nil {
				logger.Warn().Err(err).Str("role_id", roleID.String()).Msg("failed to remove role")
				failed = append(failed, discord.RoleMention(roleID))
				continue
			}
			removed = append(removed, discord.RoleMention(roleID))
			continue
		}

		if err := event.Client().Rest().AddMemberRole(*guildID, event.User().ID, roleID); err != nil {
			logger.Warn().Err(err).Str("role_id", roleID.String()).Msg("failed to add role")
			failed = append(failed, discord.RoleMention(roleID))
			continue
		}
		added = append(added, discord.RoleMention(roleID))
	}

	result := buildRolePanelResult(added, removed, failed)

	message, err := event.Client().Rest().GetMessage(event.Channel().ID(), messageID)
	if err != nil || message == nil {
		logger.Error().Err(err).Str("message_id", messageID.String()).Msg("failed to fetch role panel message for refresh")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(result).
			SetEphemeral(true).
			Build())
		return
	}

	menu, ok := findRolePanelMenu(message)
	if !ok {
		logger.Error().Str("message_id", messageID.String()).Msg("role panel menu not found for refresh")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(result).
			SetEphemeral(true).
			Build())
		return
	}

	refreshRow := discord.NewActionRow(buildRolePanelMenu(menu.CustomID, menu.Placeholder, menu.Options))
	if _, err := event.Client().Rest().UpdateMessage(event.Channel().ID(), messageID, discord.NewMessageUpdateBuilder().
		SetContainerComponents(refreshRow).
		Build()); err != nil {
		logger.Warn().Err(err).Str("message_id", messageID.String()).Msg("failed to refresh role panel selection state")
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(result).
		SetEphemeral(true).
		Build())
}

func handleRolePanelCreate(logger zerolog.Logger, rolePanels *service.RolePanelService, event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	guildID := event.GuildID()
	if guildID == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このコマンドはサーバー内でのみ使用できます。").
			SetEphemeral(true).
			Build())
		return
	}
	channelID := event.Channel().ID()
	if channel, ok := data.OptChannel("channel"); ok && channel.ID != 0 {
		channelID = channel.ID
	}
	if channelID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("チャンネルを特定できませんでした。").
			SetEphemeral(true).
			Build())
		return
	}

	title := strings.TrimSpace(data.String("title"))
	if title == "" {
		title = rolePanelDefaultTitle
	}
	description := strings.TrimSpace(data.String("description"))
	if description == "" {
		description = rolePanelDefaultDesc
	}

	resolvedRoles := rolepanel.CollectRoles(data)

	options, addedRoles, skippedRoles, overflowRoles := buildRolePanelOptions(nil, resolvedRoles)
	embed := buildRolePanelEmbed(title, description, addedRoles)

	message, err := event.Client().Rest().CreateMessage(channelID, discord.NewMessageCreateBuilder().
		SetEmbeds(embed).
		Build())
	if err != nil {
		logger.Error().Err(err).Msg("failed to create role panel message")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルの作成に失敗しました。権限とチャンネル設定を確認してください。").
			SetEphemeral(true).
			Build())
		return
	}

	menu := buildRolePanelMenu(buildRolePanelCustomID(message.ID), rolePanelDefaultPlacehold, options)
	row := discord.NewActionRow(menu)
	if _, err := event.Client().Rest().UpdateMessage(channelID, message.ID, discord.NewMessageUpdateBuilder().
		SetEmbeds(embed).
		SetContainerComponents(row).
		Build()); err != nil {
		logger.Error().Err(err).Msg("failed to set role panel components")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルの作成はできましたが、メニュー設定に失敗しました。権限を確認してください。").
			SetEphemeral(true).
			Build())
		return
	}

	if _, err := rolePanels.UpsertPanel(context.Background(), *guildID, channelID, message.ID, title, description, roleIDsFromOptions(addedRoles)); err != nil {
		logger.Warn().Err(err).Str("message_id", message.ID.String()).Msg("failed to persist role panel record")
	}

	url := discord.MessageURL(*guildID, channelID, message.ID)
	result := buildRolePanelAddResult(roleMentions(addedRoles), roleMentions(skippedRoles), nil, nil, roleMentions(overflowRoles))
	content := fmt.Sprintf("ロールパネルを作成しました: %s", url)
	if result != "" {
		content = content + "\n" + result
	}
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(content).
		SetEphemeral(true).
		Build())
}

func handleRolePanelAdd(logger zerolog.Logger, rolePanels *service.RolePanelService, event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	guildID := event.GuildID()
	if guildID == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このコマンドはサーバー内でのみ使用できます。").
			SetEphemeral(true).
			Build())
		return
	}

	panelValue := strings.TrimSpace(data.String("panel"))
	if panelValue == "" {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルを選択してください。").
			SetEphemeral(true).
			Build())
		return
	}
	panelID, err := strconv.Atoi(panelValue)
	if err != nil || panelID <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルの形式が正しくありません。").
			SetEphemeral(true).
			Build())
		return
	}

	panel, err := rolePanels.GetPanelByID(context.Background(), *guildID, panelID)
	if err != nil {
		logger.Error().Err(err).Int("panel_id", panelID).Msg("failed to load role panel record")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネル情報の取得に失敗しました。再度お試しください。").
			SetEphemeral(true).
			Build())
		return
	}

	messageID, parseErr := snowflake.Parse(panel.MessageID)
	if parseErr != nil || messageID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("保存されたメッセージIDが無効です。").
			SetEphemeral(true).
			Build())
		return
	}
	panelChannelID, parseErr := snowflake.Parse(panel.ChannelID)
	if parseErr != nil || panelChannelID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("保存されたチャンネルIDが無効です。").
			SetEphemeral(true).
			Build())
		return
	}

	resolvedRoles := rolepanel.CollectRoles(data)
	if len(resolvedRoles) == 0 {
		message := "ロールを指定してください。"
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(message).
			SetEphemeral(true).
			Build())
		return
	}

	message, err := event.Client().Rest().GetMessage(panelChannelID, messageID)
	if err != nil || message == nil {
		logger.Error().Err(err).Msg("failed to fetch role panel message")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルの取得に失敗しました。チャンネルとメッセージIDを確認してください。").
			SetEphemeral(true).
			Build())
		return
	}

	menu, ok := findRolePanelMenu(message)
	options, addedRoles, skippedRoles, overflowRoles := buildRolePanelOptions(menu.Options, resolvedRoles)
	if len(addedRoles) == 0 {
		result := buildRolePanelAddResult(nil, roleMentions(skippedRoles), nil, nil, roleMentions(overflowRoles))
		if result == "" {
			result = "追加できるロールがありませんでした。"
		}
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent(result).
			SetEphemeral(true).
			Build())
		return
	}

	customID := buildRolePanelCustomID(message.ID)
	placeholder := rolePanelDefaultPlacehold
	if ok {
		if menu.CustomID != "" && strings.HasPrefix(menu.CustomID, rolePanelCustomIDPrefix) {
			customID = menu.CustomID
		}
		if menu.Placeholder != "" {
			placeholder = menu.Placeholder
		}
	}

	updatedMenu := buildRolePanelMenu(customID, placeholder, options)
	row := discord.NewActionRow(updatedMenu)
	updatedEmbed := baseRolePanelEmbed(message)
	for _, role := range addedRoles {
		updatedEmbed = appendRolePanelEmbedField(updatedEmbed, rolePanelLabel(role), rolePanelDescription(role))
	}

	_, err = event.Client().Rest().UpdateMessage(panelChannelID, messageID, discord.NewMessageUpdateBuilder().
		SetEmbeds(updatedEmbed).
		SetContainerComponents(row).
		Build())
	if err != nil {
		logger.Error().Err(err).Msg("failed to update role panel message")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルの更新に失敗しました。権限とメッセージの状態を確認してください。").
			SetEphemeral(true).
			Build())
		return
	}

	if _, err := rolePanels.AppendRoles(context.Background(), *guildID, messageID, roleIDsFromOptions(addedRoles)); err != nil {
		logger.Warn().Err(err).Str("message_id", messageID.String()).Msg("failed to update role panel record")
	}

	result := buildRolePanelAddResult(roleMentions(addedRoles), roleMentions(skippedRoles), nil, nil, roleMentions(overflowRoles))
	if result == "" {
		result = "ロールを追加しました。"
	}
	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(result).
		SetEphemeral(true).
		Build())
}

func handleRolePanelDelete(logger zerolog.Logger, rolePanels *service.RolePanelService, event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	guildID := event.GuildID()
	if guildID == nil {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("このコマンドはサーバー内でのみ使用できます。").
			SetEphemeral(true).
			Build())
		return
	}
	panelValue := strings.TrimSpace(data.String("panel"))
	if panelValue == "" {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("削除するロールパネルを選択してください。").
			SetEphemeral(true).
			Build())
		return
	}
	panelID, err := strconv.Atoi(panelValue)
	if err != nil || panelID <= 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("削除するロールパネルの形式が正しくありません。").
			SetEphemeral(true).
			Build())
		return
	}

	panel, err := rolePanels.GetPanelByID(context.Background(), *guildID, panelID)
	if err != nil {
		logger.Error().Err(err).Int("panel_id", panelID).Msg("failed to load role panel record")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネル情報の取得に失敗しました。再度お試しください。").
			SetEphemeral(true).
			Build())
		return
	}

	messageID, parseErr := snowflake.Parse(panel.MessageID)
	if parseErr != nil || messageID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("保存されたメッセージIDが無効です。").
			SetEphemeral(true).
			Build())
		return
	}
	channelID, parseErr := snowflake.Parse(panel.ChannelID)
	if parseErr != nil || channelID == 0 {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("保存されたチャンネルIDが無効です。").
			SetEphemeral(true).
			Build())
		return
	}

	message, err := event.Client().Rest().GetMessage(channelID, messageID)
	if err != nil || message == nil {
		logger.Error().Err(err).Int("panel_id", panel.ID).Msg("failed to fetch role panel message for deletion")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルのメッセージ取得に失敗しました。既に削除されている可能性があります。").
			SetEphemeral(true).
			Build())
		return
	}

	_, hasMenu := findRolePanelMenu(message)
	if !hasMenu {
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("この記録はロールパネルではありません。").
			SetEphemeral(true).
			Build())
		return
	}

	title := panel.Title
	if title == "" && len(message.Embeds) > 0 && message.Embeds[0].Title != "" {
		title = message.Embeds[0].Title
	}
	if title == "" {
		title = rolePanelDefaultTitle
	}

	if err := event.Client().Rest().DeleteMessage(channelID, messageID); err != nil {
		logger.Error().Err(err).Int("panel_id", panel.ID).Msg("failed to delete role panel message")
		_ = event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("ロールパネルの削除に失敗しました。権限を確認してください。").
			SetEphemeral(true).
			Build())
		return
	}

	if _, err := rolePanels.DeletePanelByID(context.Background(), *guildID, panel.ID); err != nil {
		logger.Warn().Err(err).Int("panel_id", panel.ID).Msg("failed to delete role panel record")
	}

	_ = event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(fmt.Sprintf("ロールパネル「%s」を削除しました。", title)).
		SetEphemeral(true).
		Build())
}

func allowRolePanel(cfg config.Config, guildID snowflake.ID) (bool, string) {
	if !cfg.RolePanelEnabled {
		return false, "ロールパネル機能は現在無効です。設定で有効化してください。"
	}
	if len(cfg.RolePanelGuildIDs) == 0 {
		return true, ""
	}
	for _, id := range cfg.RolePanelGuildIDs {
		if strings.TrimSpace(id) == guildID.String() {
			return true, ""
		}
	}
	return false, "このサーバーではロールパネル機能が無効です。"
}

func buildRolePanelCustomID(messageID snowflake.ID) string {
	return rolePanelCustomIDPrefix + messageID.String()
}

func findRolePanelMenu(message *discord.Message) (discord.StringSelectMenuComponent, bool) {
	for _, row := range message.ActionRows() {
		for _, menu := range row.SelectMenus() {
			if selectMenu, ok := menu.(discord.StringSelectMenuComponent); ok {
				return selectMenu, true
			}
		}
	}
	return discord.StringSelectMenuComponent{}, false
}

func buildRolePanelMenu(customID string, placeholder string, options []discord.StringSelectMenuOption) discord.StringSelectMenuComponent {
	maxValues := rolePanelMaxValues(rolePanelRoleCount(options))
	return discord.NewStringSelectMenu(customID, placeholder, options...).WithMaxValues(maxValues)
}

func buildRolePanelEmbed(title string, description string, roles []discord.Role) discord.Embed {
	builder := discord.NewEmbedBuilder().
		SetTitle(title).
		SetDescription(description).
		SetColor(0x2ECC71).
		SetTimestamp(time.Now())
	if len(roles) == 0 {
		builder.AddField("登録ロール", "なし", false)
		return builder.Build()
	}
	for _, role := range roles {
		builder.AddField(rolePanelLabel(role), rolePanelDescription(role), true)
	}
	return builder.Build()
}

func baseRolePanelEmbed(message *discord.Message) discord.Embed {
	var embed discord.Embed
	if message != nil && len(message.Embeds) > 0 {
		embed = message.Embeds[0]
	} else {
		embed = discord.NewEmbedBuilder().
			SetTitle(rolePanelDefaultTitle).
			SetDescription(rolePanelDefaultDesc).
			SetColor(0x2ECC71).
			Build()
	}
	fields := make([]discord.EmbedField, 0, len(embed.Fields))
	for _, field := range embed.Fields {
		if field.Name == "登録ロール" && field.Value == "なし" {
			continue
		}
		fields = append(fields, field)
	}
	embed.Fields = fields
	return embed
}

func appendRolePanelEmbedField(embed discord.Embed, label string, description string) discord.Embed {
	fields := make([]discord.EmbedField, 0, len(embed.Fields)+1)
	fields = append(fields, embed.Fields...)
	inline := true
	fields = append(fields, discord.EmbedField{Name: label, Value: description, Inline: &inline})
	embed.Fields = fields
	return embed
}

func buildRolePanelOptions(existing []discord.StringSelectMenuOption, addRoles []discord.Role) ([]discord.StringSelectMenuOption, []discord.Role, []discord.Role, []discord.Role) {
	options, existingValues := normalizeRolePanelOptions(existing)
	added := make([]discord.Role, 0, len(addRoles))
	skipped := make([]discord.Role, 0)
	overflow := make([]discord.Role, 0)
	roleCount := rolePanelRoleCount(options)
	for _, role := range addRoles {
		value := role.ID.String()
		if _, ok := existingValues[value]; ok {
			skipped = append(skipped, role)
			continue
		}
		if roleCount >= rolePanelMaxRoles {
			overflow = append(overflow, role)
			continue
		}
		options = append(options, buildRolePanelOption(role))
		existingValues[value] = struct{}{}
		added = append(added, role)
		roleCount++
	}
	return options, added, skipped, overflow
}

func normalizeRolePanelOptions(existing []discord.StringSelectMenuOption) ([]discord.StringSelectMenuOption, map[string]struct{}) {
	options := make([]discord.StringSelectMenuOption, 0, rolePanelMaxOptions)
	values := make(map[string]struct{}, len(existing))
	for _, option := range existing {
		if option.Value == rolePanelPlaceholderValue {
			continue
		}
		if _, ok := values[option.Value]; ok {
			continue
		}
		values[option.Value] = struct{}{}
		options = append(options, option)
	}
	return options, values
}

func buildRolePanelOption(role discord.Role) discord.StringSelectMenuOption {
	label := rolePanelLabel(role)
	description := rolePanelDescription(role)
	return discord.NewStringSelectMenuOption(label, role.ID.String()).WithDescription(description)
}

func rolePanelLabel(role discord.Role) string {
	return truncateRolePanelText(role.Name, 100)
}

func rolePanelDescription(role discord.Role) string {
	if role.Description != nil {
		desc := strings.TrimSpace(*role.Description)
		if desc != "" {
			return truncateRolePanelText(desc, 100)
		}
	}
	return role.Mention()
}

func rolePanelRoleCount(options []discord.StringSelectMenuOption) int {
	count := 0
	for _, option := range options {
		if option.Value == rolePanelPlaceholderValue {
			continue
		}
		count++
	}
	return count
}

func rolePanelMaxValues(roleCount int) int {
	if roleCount < 1 {
		return 1
	}
	if roleCount > rolePanelMaxRoles {
		return rolePanelMaxRoles
	}
	return roleCount
}

func roleMentions(roles []discord.Role) []string {
	mentions := make([]string, 0, len(roles))
	for _, role := range roles {
		mentions = append(mentions, role.Mention())
	}
	return mentions
}

func roleIDsFromOptions(roles []discord.Role) []snowflake.ID {
	ids := make([]snowflake.ID, 0, len(roles))
	for _, role := range roles {
		if role.ID == 0 {
			continue
		}
		ids = append(ids, role.ID)
	}
	return ids
}

func filterRolePanelValues(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value == rolePanelPlaceholderValue {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func memberHasRole(member *discord.Member, roleID snowflake.ID) bool {
	if member == nil {
		return false
	}
	for _, id := range member.RoleIDs {
		if id == roleID {
			return true
		}
	}
	return false
}

func truncateRolePanelText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit]
}

func buildRolePanelAddResult(added []string, skipped []string, missing []string, invalid []string, overflow []string) string {
	parts := make([]string, 0, 5)
	if len(added) > 0 {
		parts = append(parts, "追加: "+strings.Join(added, ", "))
	}
	if len(skipped) > 0 {
		parts = append(parts, "既に追加済み: "+strings.Join(skipped, ", "))
	}
	if len(missing) > 0 {
		parts = append(parts, "見つからないロール: "+strings.Join(missing, ", "))
	}
	if len(invalid) > 0 {
		parts = append(parts, "無効な入力: "+strings.Join(invalid, ", "))
	}
	if len(overflow) > 0 {
		parts = append(parts, "上限超過: "+strings.Join(overflow, ", "))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func buildRolePanelResult(added []string, removed []string, failed []string) string {
	parts := make([]string, 0, 3)
	if len(added) > 0 {
		parts = append(parts, "付与: "+strings.Join(added, ", "))
	}
	if len(removed) > 0 {
		parts = append(parts, "解除: "+strings.Join(removed, ", "))
	}
	if len(failed) > 0 {
		parts = append(parts, "失敗: "+strings.Join(failed, ", "))
	}
	if len(parts) == 0 {
		return "変更はありませんでした。"
	}
	return strings.Join(parts, "\n")
}
