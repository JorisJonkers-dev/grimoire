package domain

import (
	"time"

	"github.com/google/uuid"
)

// EntityType names a kind of prep data that keeps Revisions.
type EntityType string

// Revisioned entity types.
const (
	EntityNPC       EntityType = "npc"
	EntityCharacter EntityType = "character"
)

// RevisionAction is what a Revision recorded.
type RevisionAction string

// Revision actions.
const (
	ActionCreate  RevisionAction = "create"
	ActionUpdate  RevisionAction = "update"
	ActionDelete  RevisionAction = "delete"
	ActionRestore RevisionAction = "restore"
)

// Revision is one recorded version of a piece of prep data.
type Revision struct {
	No           int
	Action       RevisionAction
	Author       string
	Origin       string
	Client       string
	RestoredFrom int
	CreatedAt    time.Time
}

// Field is one named value of a snapshot, in display order.
type Field struct {
	Name  string
	Value string
}

// Change is a field that differs between two snapshots.
type Change struct {
	Field  string
	Before string
	After  string
}

// Diff lists the fields whose values differ, in the order of before.
func Diff(before, after []Field) []Change {
	next := make(map[string]string, len(after))
	for _, f := range after {
		next[f.Name] = f.Value
	}
	out := []Change{}
	for _, f := range before {
		if next[f.Name] != f.Value {
			out = append(out, Change{Field: f.Name, Before: f.Value, After: next[f.Name]})
		}
	}
	return out
}

// NPCID identifies an NPC.
type NPCID uuid.UUID

// NPC is a non-player character in a Campaign's prep.
type NPC struct {
	ID          NPCID
	Name        string
	Title       string
	Description string
	DMNotes     string
	Disposition string
	UpdatedAt   time.Time
}

// Fields is the NPC as a snapshot for comparison.
func (n NPC) Fields() []Field {
	return []Field{
		{Name: "name", Value: n.Name},
		{Name: "title", Value: n.Title},
		{Name: "description", Value: n.Description},
		{Name: "dmNotes", Value: n.DMNotes},
		{Name: "disposition", Value: n.Disposition},
	}
}

// DeletedNPC is an NPC that only its Revisions remember.
type DeletedNPC struct {
	ID        NPCID
	Name      string
	DeletedAt time.Time
}

// Edit is one prep change as a Revision: what it changed, and whether it is still that entity's latest.
type Edit struct {
	Revision
	RevisionID uuid.UUID
	EntityType EntityType
	EntityID   uuid.UUID
	Name       string
	Latest     bool
}

// EditFilter narrows a list of Edits; zero fields match everything.
type EditFilter struct {
	Origin     string
	RevisionID uuid.UUID
	EntityType EntityType
	EntityID   uuid.UUID
	Limit      int
}
