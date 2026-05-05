package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type RelationTuple struct {
	ent.Schema
}

func (RelationTuple) Fields() []ent.Field {
	return []ent.Field{
		field.String("subject").NotEmpty(),
		field.String("relation").NotEmpty(),
		field.String("object").NotEmpty(),
		field.Time("createdAt").Default(time.Now),
	}
}

func (RelationTuple) Edges() []ent.Edge {
	return nil
}
