package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/rolltable"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// RollTableBuild is a Roll Table in its builder: its entry, its design and how it reads back.
type RollTableBuild struct {
	Entry  domain.Entry
	Design rolltable.Design
	Lines  []string
}

// firstRollTable is where a new Roll Table starts: a d20 with nothing on it yet.
func firstRollTable() rolltable.Design {
	return rolltable.Design{Dice: "1d20", Results: []rolltable.Result{}}
}

func checkRollTable(d rolltable.Design) error {
	if err := rolltable.Check(d); err != nil {
		return apperr.Refuse(err.Error())
	}
	return nil
}

// RollTable reads a Roll Table in the builder: one of the caller's, or a Shared Library copy.
func (s *Service) RollTable(ctx context.Context, c caller.Caller, id uuid.UUID) (RollTableBuild, error) {
	e, err := s.designed(ctx, c, id, "table", "Roll Table builder")
	if err != nil {
		return RollTableBuild{}, err
	}
	d := designOf(e, firstRollTable())
	return RollTableBuild{Entry: e, Design: d, Lines: rolltable.Lines(d)}, nil
}

// SaveRollTable saves a design for one of the caller's tables as its next Revision.
func (s *Service) SaveRollTable(ctx context.Context, c caller.Caller, id uuid.UUID, d rolltable.Design) (RollTableBuild, error) {
	e, err := s.ownDesigned(ctx, c, id, "table", "Roll Table builder")
	if err == nil {
		err = checkRollTable(d)
	}
	if err == nil {
		err = s.saveDesign(ctx, c, e, d)
	}
	if err != nil {
		return RollTableBuild{}, err
	}
	return s.RollTable(ctx, c, id)
}

// PreviewRollTable checks a design without saving it and reads it back.
func (s *Service) PreviewRollTable(d rolltable.Design) (RollTableBuild, error) {
	if err := checkRollTable(d); err != nil {
		return RollTableBuild{}, err
	}
	return RollTableBuild{Design: d, Lines: rolltable.Lines(d)}, nil
}
