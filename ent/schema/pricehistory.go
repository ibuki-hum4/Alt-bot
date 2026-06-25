package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
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

func (PriceHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
	}
}
