package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Token holds the schema definition for the Token entity.
type Token struct {
	ent.Schema
}

// Fields of the Token.
func (Token) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.Uint32("user_id"),
		field.String("name").
			NotEmpty(),
		field.String("token_hash").
			NotEmpty().
			Sensitive(),
		field.Time("created_at").
			Default(time.Now),
		field.Time("last_used_at").
			Default(time.Now).
			UpdateDefault(time.Now),
		field.Time("expires_at"),
		field.Time("revoked_at").
			Optional().
			Nillable(),
	}
}

// Edges of the Token.
func (Token) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("tokens").
			Field("user_id").
			Unique().
			Required(),
	}
}