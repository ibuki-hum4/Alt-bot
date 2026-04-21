package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type PriceHistory struct {
	ent.Schema
}

func (PriceHistory) Fields() []ent.Field {
	return []ent.Field{
		field.Float("price"),
		field.Time("created_at").
			Default(time.Now),
	}
}
