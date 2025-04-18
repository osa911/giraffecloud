package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("id"),
		field.String("firebase_uid").Unique(),
		field.String("email").Unique(),
		field.String("name").Optional().Default(""),
		field.String("role").Default("user"),
		field.Bool("is_active").Default(true),
		field.Time("last_login").Optional().Nillable(),
		field.String("last_login_ip").Optional().Nillable(),
		field.Time("last_activity").Optional().Nillable(),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sessions", Session.Type),
		edge.To("tokens", Token.Type),
	}
}

// Mixin for the User schema.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Mixin{},
	}
}
