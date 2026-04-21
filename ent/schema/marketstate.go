package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type MarketState struct {
	ent.Schema
}

func (MarketState) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Unique(),
		field.String("current_event").
			Default(""),
		field.Int("event_remaining_ticks").
			Default(0),
		field.Int("pity_counter").
			Default(0),
		field.String("circuit_level").
			Default("NORMAL"),
		field.Time("last_breach_at").
			Optional().
			Nillable(),
		field.Float("last_price").
			Default(100),
		field.Int64("reserve_balance").
			Default(0),
		field.Int64("revenue_balance").
			Default(0),
		field.Int64("burned_total").
			Default(0),
		field.String("daily_issuance_date").
			Default(""),
		field.Int64("daily_issued").
			Default(0),
		field.Int64("daily_cap").
			Default(20000),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}
