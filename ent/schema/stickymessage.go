package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// StickyMessage stores a message the bot keeps at the bottom of a channel by
// deleting its previous copy and reposting it whenever someone talks.
//
// The configuration lives in the database rather than in the config file so
// the web dashboard can read and change it at runtime.
type StickyMessage struct {
	ent.Schema
}

func (StickyMessage) Fields() []ent.Field {
	return []ent.Field{
		field.String("guild_id").
			NotEmpty(),
		field.String("channel_id").
			NotEmpty(),
		field.Text("content").
			NotEmpty(),
		// last_message_id is the copy currently posted in the channel, or ""
		// when nothing has been posted yet. It survives restarts so the old
		// copy can still be cleaned up after the process dies.
		field.String("last_message_id").
			Default(""),
		// enabled lets the dashboard pause a sticky without losing its text.
		field.Bool("enabled").
			Default(true),
		field.String("created_by").
			Default(""),
		field.Time("created_at").
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (StickyMessage) Indexes() []ent.Index {
	return []ent.Index{
		// One sticky per channel; a channel only ever belongs to one guild.
		index.Fields("channel_id").
			Unique(),
		index.Fields("guild_id"),
	}
}
