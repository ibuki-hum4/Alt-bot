package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TransactionLog stores signed and chained transaction records.
type TransactionLog struct {
	ent.Schema
}

func (TransactionLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("tx_id").
			NotEmpty().
			Unique(),
		field.String("user_discord_id").
			NotEmpty(),
		field.String("kind").
			NotEmpty(),
		field.Int64("yen_delta").
			Default(0),
		field.Int64("alt_delta").
			Default(0),
		field.Int64("xp_delta").
			Default(0),
		field.Int64("amount").
			Default(0),
		field.Float("settled_price").
			Default(0),
		field.Float("price_after").
			Default(0),
		field.Int64("balance_after").
			Default(0),
		field.Int64("crypto_after").
			Default(0),
		field.String("prev_hash").
			Default(""),
		field.String("hash").
			NotEmpty(),
		field.String("signature").
			NotEmpty(),
		field.String("public_key").
			NotEmpty(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (TransactionLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
		index.Fields("user_discord_id", "created_at"),
		index.Fields("kind", "created_at"),
	}
}
