package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// ChainState is a singleton row holding the current tip hash of the
// TransactionLog hash chain. It is locked with ForUpdate() inside
// appendSignedLogChain so concurrent transactions serialize their chain
// appends at the database level, replacing the process-wide
// EconomyService mutex plus in-memory prevHash field that used to keep the
// chain linear.
type ChainState struct {
	ent.Schema
}

func (ChainState) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			Unique(),
		field.String("latest_hash").
			Default(""),
	}
}
