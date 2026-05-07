package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RolePanel stores the persisted metadata for a Discord role panel message.
type RolePanel struct {
	ent.Schema
}

func (RolePanel) Fields() []ent.Field {
	return []ent.Field{
		field.String("guild_id").
			NotEmpty(),
		field.String("channel_id").
			NotEmpty(),
		field.String("message_id").
			NotEmpty(),
		field.String("title").
			NotEmpty(),
		field.Text("description").
			Default(""),
		field.Text("role_ids").
			Default(""),
		field.Time("created_at").
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (RolePanel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("guild_id", "message_id").
			Unique(),
		index.Fields("guild_id", "title"),
		index.Fields("guild_id", "updated_at"),
	}
}