package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ClientVersion holds the schema definition for the ClientVersion entity.
type ClientVersion struct {
	ent.Schema
}

// Fields of the ClientVersion.
func (ClientVersion) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Unique().
			Comment("Unique identifier for this version config"),

		field.String("channel").
			Default("stable").
			Comment("Release channel: stable, beta, test"),

		field.String("platform").
			Default("all").
			Comment("Target platform: all, linux, darwin, windows"),

		field.String("arch").
			Default("all").
			Comment("Target architecture: all, amd64, arm64"),

		field.String("latest_version").
			Comment("Latest available version for this channel"),

		field.String("minimum_version").
			Comment("Minimum required version"),

		field.String("download_url").
			Comment("Base download URL for this channel"),

		field.String("release_notes").
			Optional().
			Comment("Release notes for the latest version"),

		field.Bool("auto_update_enabled").
			Default(true).
			Comment("Whether auto-update is enabled for this channel"),

		field.Bool("force_update").
			Default(false).
			Comment("Force update even if user disabled auto-update"),

		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			Comment("Additional metadata for the version"),

		field.Time("created_at").
			Default(time.Now).
			Immutable(),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

// Edges of the ClientVersion.
func (ClientVersion) Edges() []ent.Edge {
	return nil
}

// Indexes of the ClientVersion.
func (ClientVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel", "platform", "arch").
			Unique(),
	}
}