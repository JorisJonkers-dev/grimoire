package live

import (
	"maps"
	"slices"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/reactions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// reactionKinds are the kinds of Reaction Prompt a Controller sets.
var reactionKinds = []string{domain.PromptOpportunity, domain.PromptShield, domain.PromptReadied, domain.PromptEffect} //nolint:gochecknoglobals // a fixed list

// offers reports whether a token's Controller wants a kind of reaction offered at all.
func (s *state) offers(t domain.Token, kind string) bool {
	return reactions.Decide(t.Reactions[kind], reactions.Situation{TargetHP: 0, TargetHPMax: 0}).Offer
}

// planReaction stores a Controller's setting for one kind of reaction.
func (r *runtime) planReaction(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	set := reactions.Setting{Mode: reactions.Mode(cmd.ReactionMode), Condition: reactions.Condition(cmd.Condition)}
	switch {
	case !ok || t.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	case !slices.Contains(reactionKinds, cmd.ReactionKind) || !reactions.Valid(set):
		return Write{}, "Choose a reaction and whether to ask, always take it, or never."
	}
	t.Reactions = maps.Clone(t.Reactions)
	if t.Reactions == nil {
		t.Reactions = map[string]reactions.Setting{}
	}
	t.Reactions[cmd.ReactionKind] = set
	return Write{Kind: domain.ActionReactionSet, Token: t}, ""
}

// autoReact answers a prompt the write just raised when its Controller always takes that reaction and
// the setting's condition holds; it reports whether it answered.
func (r *runtime) autoReact(w Write, actor domain.Member, c caller.Caller) bool {
	f := r.st.combat
	if w.prompt == nil || f == nil || f.Prompt == nil || f.Prompt.ID != w.prompt.ID {
		return false
	}
	p := f.Prompt
	reactor, target := r.st.tokens[p.Reactor], r.st.tokens[p.Trigger]
	at := reactions.Situation{TargetHP: 0, TargetHPMax: 0}
	if target.Stats != nil {
		at = reactions.Situation{TargetHP: target.Stats.HP, TargetHPMax: target.Stats.HPMax}
	}
	if !reactions.Decide(reactor.Reactions[p.Kind], at).Auto {
		return false
	}
	r.commit(request{}, r.answer(p, true, actor), actor, c)
	return true
}

// reactToDamage offers the reaction an Effect gives a creature that was just damaged (Hellish Rebuke).
func (r *runtime) reactToDamage(t, by domain.Token, dealt int, actor domain.Member, c caller.Caller) {
	f := r.st.combat
	if dealt <= 0 || f == nil || f.Prompt != nil || f.Attack != nil {
		return
	}
	t = r.st.tokens[t.ID]
	x, ok := r.st.fighter(t.ID)
	given := r.st.catalog.ReactionsTo(r.st.actives(t.ID), "damaged")
	if !ok || len(given) == 0 || !x.Economy.Reaction || !standing(t) || r.st.catalog.Incapacitated(r.st.actives(t.ID)) || !r.st.offers(t, domain.PromptEffect) {
		return
	}
	pr := r.prompt(domain.PromptEffect, t, by, 0, given[0].Instruction)
	r.commit(request{}, Write{Kind: domain.ActionReactionOffered, Token: t, prompt: pr}, actor, c)
}

// ReactionSettingView is a token's setting for one kind of reaction.
type ReactionSettingView struct {
	Kind      string `json:"kind"`
	Mode      string `json:"mode"`
	Condition string `json:"condition,omitempty"`
}

// reactionViews lists a token's reaction settings in a fixed order.
func reactionViews(t domain.Token) []ReactionSettingView {
	var out []ReactionSettingView
	for _, kind := range reactionKinds {
		if set, ok := t.Reactions[kind]; ok {
			out = append(out, ReactionSettingView{Kind: kind, Mode: string(set.Mode), Condition: string(set.Condition)})
		}
	}
	return out
}
