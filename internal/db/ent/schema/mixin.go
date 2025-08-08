package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// Mixin implements the ent.Mixin for sharing time fields with package schemas.
type Mixin struct {
	mixin.Schema
}

// Fields of the Mixin.
func (Mixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").
			Immutable().
			Default(time.Now),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (Mixin) Annotations() []schema.Annotation {
	return nil
}

// Edges of the Mixin.
func (Mixin) Edges() []ent.Edge {
	return nil
}

// Hooks of the Mixin.
func (Mixin) Hooks() []ent.Hook {
	return nil
}

// Indexes of the Mixin.
func (Mixin) Indexes() []ent.Index {
	return nil
}

// Interceptors of the Mixin.
func (Mixin) Interceptors() []ent.Interceptor {
	return nil
}

// Policy of the Mixin.
func (Mixin) Policy() ent.Policy {
	return nil
}
