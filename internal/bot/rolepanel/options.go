package rolepanel

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

const RoleOptionCount = 10

func RoleOptions() []discord.ApplicationCommandOption {
	options := make([]discord.ApplicationCommandOption, 0, RoleOptionCount)
	for index := 1; index <= RoleOptionCount; index++ {
		options = append(options, discord.ApplicationCommandOptionRole{
			Name:        RoleOptionName(index),
			Description: fmt.Sprintf("ロール%d", index),
			Required:    false,
		})
	}
	return options
}

func RoleOptionName(index int) string {
	return fmt.Sprintf("role%d", index)
}

func CollectRoles(data discord.SlashCommandInteractionData) []discord.Role {
	roles := make([]discord.Role, 0, RoleOptionCount)
	seen := make(map[snowflake.ID]struct{}, RoleOptionCount)
	for index := 1; index <= RoleOptionCount; index++ {
		role, ok := data.OptRole(RoleOptionName(index))
		if !ok || role.ID == 0 {
			continue
		}
		if _, exists := seen[role.ID]; exists {
			continue
		}
		seen[role.ID] = struct{}{}
		roles = append(roles, role)
	}
	return roles
}