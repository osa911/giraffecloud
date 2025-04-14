package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Session holds the schema definition for the Session entity.
type Session struct {
	ent.Schema
}

// Fields of the Session.
func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("id"),
		field.String("token").Unique(),
		field.Time("expires_at"),
		field.Time("last_used"),
		field.Bool("is_active").Default(true),
		field.String("user_agent").Optional(),
		field.String("ip_address").Optional(),
	}
}

// Edges of the Session.
func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).
			Ref("sessions").
			Unique().
			Required(),
	}
}

// Mixin for the Session schema.
func (Session) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Mixin{},
	}
}
