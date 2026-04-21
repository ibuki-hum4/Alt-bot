package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User is a global user model shared across all guilds.
type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("discord_id").
			NotEmpty().
			Unique(),
		field.Int64("balance").
			Default(0),
		field.Int64("crypto_balance").
			Default(0),
		field.Int64("xp").
			Default(0),
		field.Time("work_end_at").
			Default(time.Unix(0, 0).UTC()),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("discord_id"),
	}
}
