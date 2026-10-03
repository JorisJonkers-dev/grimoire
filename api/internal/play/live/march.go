package live

import (
	"cmp"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// marchPlace is a token's place in the Marching Order, from 1 at the front; 0 when it has none.
func (s *state) marchPlace(t domain.Token) int {
	id, ok := characterOf(t)
	if !ok {
		return 0
	}
	return slices.Index(s.march, id) + 1
}

// marchView lists every Character of the Campaign: those placed in the Marching Order first, from the
// front, then the rest by name.
func (s *state) marchView() []MarchView {
	out := make([]MarchView, 0, len(s.inventory.Bearers))
	for _, b := range s.inventory.Bearers {
		out = append(out, MarchView{CharacterID: b.CharacterID.String(), Name: b.Name, Place: slices.Index(s.march, b.CharacterID) + 1})
	}
	last := len(out) + 1
	place := func(m MarchView) int {
		if m.Place == 0 {
			return last
		}
		return m.Place
	}
	slices.SortFunc(out, func(a, b MarchView) int {
		return cmp.Or(cmp.Compare(place(a), place(b)), strings.Compare(a.Name, b.Name), strings.Compare(a.CharacterID, b.CharacterID))
	})
	return out
}

// planMarch arranges the Marching Order: the Characters named, from the front, each once. Anyone at the
// table may arrange it.
func (r *runtime) planMarch(cmd Command) (Write, string) {
	order := make([]uuid.UUID, 0, len(cmd.CharacterIDs))
	for _, raw := range cmd.CharacterIDs {
		id, err := uuid.Parse(raw)
		switch {
		case err != nil || !slices.ContainsFunc(r.st.inventory.Bearers, func(b domain.Bearer) bool { return b.CharacterID == id }):
			return Write{}, "Only Characters of this Campaign march."
		case slices.Contains(order, id):
			return Write{}, "A Character has a place in the Marching Order once."
		}
		order = append(order, id)
	}
	return Write{Kind: domain.ActionMarchingOrderSet, March: order}, ""
}
