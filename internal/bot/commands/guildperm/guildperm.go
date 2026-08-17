// Package guildperm holds the guild permission checks shared by the
// administrative commands, so /mod and /rp enforce the same rules with the
// same wording instead of each re-implementing the check locally.
package guildperm

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// CheckManageGuild verifies the interaction came from inside a guild and that
// the invoking member holds the Manage Server permission. It returns the
// guild ID so callers do not have to re-check that event.GuildID() is
// non-nil before dereferencing it.
//
// ok is false when the check fails, and message is then the user-facing
// Japanese explanation the caller should reply with (ephemerally).
//
// The command definitions already set DefaultMemberPermissions, but a guild
// can override that in Discord's integration settings, so commands verify
// the permission again here rather than trusting the client.
func CheckManageGuild(event *events.ApplicationCommandInteractionCreate) (guildID snowflake.ID, message string, ok bool) {
	id := event.GuildID()
	if id == nil {
		return 0, "このコマンドはサーバー内でのみ利用できます。", false
	}
	member := event.Member()
	if member == nil {
		return 0, "メンバー情報を取得できません。", false
	}
	if member.Permissions&discord.PermissionManageGuild == 0 {
		return 0, "このコマンドはサーバー管理者のみが実行できます。", false
	}
	return *id, "", true
}
