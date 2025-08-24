package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Plan holds billing/limits plan configuration.
type Plan struct {
	ent.Schema
}

func (Plan) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().Unique(),
		field.Int64("monthly_limit_bytes").Default(100 * 1024 * 1024 * 1024), // 100GB
		field.Int("overage_per_gb_cents").Default(0),
		field.Bool("active").Default(true),
	}
}

func (Plan) Mixin() []ent.Mixin { return []ent.Mixin{Mixin{}} }
