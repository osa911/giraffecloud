package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Tunnel holds the schema definition for the Tunnel entity.
type Tunnel struct {
	ent.Schema
}

// Fields of the Tunnel.
func (Tunnel) Fields() []ent.Field {
	return []ent.Field{
		field.String("domain").
			NotEmpty().
			Unique(),
		field.String("target_host").
			Default("localhost").
			StructTag(`json:"target_host"`),
		field.String("client_ip").
			Optional(),
		field.Bool("is_enabled").
			Default(true).
			StructTag(`json:"is_enabled"`),
		field.Enum("dns_propagation_status").
			Values("verified", "pending_dns").
			Default("verified").
			StructTag(`json:"dns_propagation_status"`),
		field.Int("target_port").
			Positive(),
		field.Uint32("user_id"),
	}
}

// Edges of the Tunnel.
func (Tunnel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).
			Ref("tunnels").
			Field("user_id").
			Unique().
			Required(),
	}
}

// Indexes of the Tunnel.
func (Tunnel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "target_host", "target_port").Unique(),
	}
}

// Mixin of the Tunnel.
func (Tunnel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Mixin{},
	}
}
