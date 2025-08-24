package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Usage holds daily aggregated traffic usage per user/tunnel/domain.
type Usage struct {
	ent.Schema
}

// Fields of the Usage.
func (Usage) Fields() []ent.Field {
	return []ent.Field{
		field.Time("period_start"),
		field.Uint32("user_id"),
		field.Uint32("tunnel_id").Optional(),
		field.String("domain").Default(""),
		field.Int64("bytes_in").Default(0),
		field.Int64("bytes_out").Default(0),
		field.Int64("requests").Default(0),
	}
}

// Mixin of the Usage.
func (Usage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Mixin{},
	}
}

// Indexes defines unique composite index to ensure idempotent aggregation
func (Usage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("period_start", "user_id", "tunnel_id", "domain").Unique(),
	}
}
