package live

import (
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
)

// perceived works out what the party makes of a token. The party always knows its own; anything else
// must get past its Visibility Qualities by some standing party member's Senses, or by plain eyes when
// no party member is on the board.
func (s *state) perceived(t domain.Token) vision.Result {
	if t.Kind == domain.TokenParty {
		return vision.Result{Seen: true, TrueForm: true}
	}
	qualities, beaten := s.qualities(t)
	if len(qualities) == 0 {
		return vision.Result{Seen: true, TrueForm: true}
	}
	at := hex.Coord{Q: t.Q, R: t.R}
	out, members := vision.Result{Seen: false, TrueForm: false}, 0
	for _, p := range s.tokens {
		if p.Kind != domain.TokenParty || !standing(p) {
			continue
		}
		members++
		target := vision.Target{Qualities: qualities, Beaten: beaten, DistanceFt: hex.Distance(at, hex.Coord{Q: p.Q, R: p.R}) * hex.FeetPerHex, OnGround: true}
		got := vision.Perceive(perceivers(p), target)
		out.Seen, out.TrueForm = out.Seen || got.Seen, out.TrueForm || got.TrueForm
	}
	if members == 0 {
		return vision.Perceive([]vision.Perceiver{{Sense: vision.Sight, RangeFt: 0}}, vision.Target{Qualities: qualities, Beaten: beaten, DistanceFt: 0, OnGround: true})
	}
	return out
}

// qualities lists a token's Visibility Qualities, with the obscurement of a cloud it stands in and
// Invisible while the Invisible condition is on it, and the ones the party has beaten with a check.
func (s *state) qualities(t domain.Token) ([]vision.Quality, []vision.Quality) {
	var out, beaten []vision.Quality
	for _, q := range slices.Sorted(maps.Keys(t.Qualities)) {
		out = append(out, vision.Quality(q))
		if t.Qualities[q] {
			beaten = append(beaten, vision.Quality(q))
		}
	}
	if o := s.terrainKinds.Obscures(s.surfaces[hex.Coord{Q: t.Q, R: t.R}].Kind); o != surface.Clear && !t.Qualities[string(o)] {
		out = append(out, vision.Quality(o))
	}
	if t.Qualities[string(vision.Invisible)] || !slices.ContainsFunc(s.actives(t.ID), func(a effects.Active) bool { return a.Slug == string(vision.Invisible) }) {
		return out, beaten
	}
	return append(out, vision.Invisible), beaten
}

// perceivers are a party member's Senses: its eyes, its darkvision, and what its statblock adds.
func perceivers(t domain.Token) []vision.Perceiver {
	out := []vision.Perceiver{{Sense: vision.Sight, RangeFt: 0}}
	if t.DarkvisionFt > 0 {
		out = append(out, vision.Perceiver{Sense: vision.Darkvision, RangeFt: t.DarkvisionFt})
	}
	for _, sense := range []vision.Sense{vision.Blindsight, vision.Tremorsense, vision.Truesight} {
		if ft := t.Stats.Senses[string(sense)]; ft > 0 {
			out = append(out, vision.Perceiver{Sense: sense, RangeFt: ft})
		}
	}
	return out
}

// masked is a token as an audience may know it: a Disguised one keeps its disguise until the party
// sees its true form.
func (s *state) masked(t domain.Token, a Audience) domain.Token {
	if a == AudienceDM || t.Disguise == "" || !slices.Contains(slices.Collect(maps.Keys(t.Qualities)), string(vision.Disguised)) || s.perceived(t).TrueForm {
		return t
	}
	t.Label, t.Form = t.Disguise, nil
	return t
}

// planVisibility sets a token's Visibility Qualities, which of them the party has seen through, and
// the name a Disguised token shows.
func (r *runtime) planVisibility(cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	if !ok {
		return Write{}, "No such token."
	}
	known := vision.Qualities()
	qualities := map[string]bool{}
	for _, q := range cmd.Qualities {
		if !slices.Contains(known, vision.Quality(q)) {
			return Write{}, "Choose Qualities from: " + strings.Join(names(known), ", ") + "."
		}
		qualities[q] = false
	}
	for _, q := range cmd.SeenThrough {
		if _, has := qualities[q]; !has {
			return Write{}, "Only a Quality the token has can be seen through."
		}
		qualities[q] = true
	}
	disguise := strings.TrimSpace(cmd.Disguise)
	_, disguised := qualities[string(vision.Disguised)]
	switch {
	case disguised && (disguise == "" || len([]rune(disguise)) > 40):
		return Write{}, "A Disguised token needs the name it shows, up to 40 characters."
	case !disguised:
		disguise = ""
	}
	t.Qualities, t.Disguise = qualities, disguise
	if len(qualities) == 0 {
		t.Qualities = nil
	}
	return Write{Kind: domain.ActionVisibilitySet, Token: t}, ""
}

func names(qs []vision.Quality) []string {
	out := make([]string, 0, len(qs))
	for _, q := range qs {
		out = append(out, string(q))
	}
	return out
}

// revealed strips the named Qualities from every token in the hexes, and lists the tokens it changed.
func (s *state) revealed(hexes []hex.Coord, strip []string) []domain.Token {
	if len(strip) == 0 {
		return nil
	}
	var out []domain.Token
	for _, t := range s.tokens {
		if !slices.Contains(hexes, hex.Coord{Q: t.Q, R: t.R}) {
			continue
		}
		left := maps.Clone(t.Qualities)
		for _, q := range strip {
			delete(left, q)
		}
		if len(left) == len(t.Qualities) {
			continue
		}
		if _, still := left[string(vision.Disguised)]; !still {
			t.Disguise = ""
		}
		t.Qualities = left
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// unveil strips the Qualities an Effect reveals from the token it lands on.
func unveil(w *Write, strip []string) {
	t := w.Token
	left := maps.Clone(t.Qualities)
	for _, q := range strip {
		delete(left, q)
	}
	if len(left) == len(t.Qualities) {
		return
	}
	if _, still := left[string(vision.Disguised)]; !still {
		t.Disguise = ""
	}
	t.Qualities = left
	w.Token, w.Unveiled = t, true
}
