package live

import (
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

type heard struct {
	notices []Notice
	members []uuid.UUID
}

func (h *heard) Notify(_, member uuid.UUID, n Notice) {
	h.notices = append(h.notices, n)
	h.members = append(h.members, member)
}

func TestPlayersHearAboutTheirTurnAndTheirReactions(t *testing.T) {
	t.Parallel()
	player, campaign, sid := uuid.New(), uuid.New(), domain.SessionID(uuid.New())
	aria := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Aria", Controller: &player}
	goblin := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Goblin"}
	ten, five := 10, 5
	fight := func(turn int, prompt *domain.ReactionPrompt) *state {
		return &state{
			session: domain.Session{ID: sid},
			tokens:  map[domain.TokenID]domain.Token{aria.ID: aria, goblin.ID: goblin},
			combat: &domain.Combat{Status: domain.CombatActive, Turn: turn, Prompt: prompt, Combatants: []domain.Combatant{
				{TokenID: aria.ID, Initiative: &ten}, {TokenID: goblin.ID, Initiative: &five},
			}},
		}
	}
	h := &heard{}
	r := &runtime{campaign: campaign, notify: h}
	calm := &state{session: domain.Session{ID: sid}, tokens: fight(10, nil).tokens}
	r.nudge(calm, fight(10, nil))
	r.nudge(fight(10, nil), fight(10, nil))
	r.nudge(fight(10, nil), fight(5, nil))
	shield := &domain.ReactionPrompt{ID: uuid.New(), Reactor: aria.ID, Effect: "Shield: AC 15 → 20."}
	r.nudge(fight(5, nil), fight(5, shield))
	r.nudge(fight(5, shield), fight(5, shield))
	r.nudge(fight(5, nil), fight(5, &domain.ReactionPrompt{ID: uuid.New(), Reactor: goblin.ID}))
	r.nudge(calm, fight(5, shield))
	url := "/campaigns/" + campaign.String() + "/sessions/" + uuid.UUID(sid).String()
	want := []Notice{{Title: "Your turn", Body: "Aria is up.", URL: url}, {Title: "Reaction", Body: "Shield: AC 15 → 20.", URL: url}}
	if len(h.notices) != 3 || h.notices[0] != want[0] || h.notices[1] != want[1] || h.notices[2] != want[1] || h.members[0] != player {
		t.Fatalf("notices = %+v", h.notices)
	}
	(&runtime{}).nudge(calm, fight(10, nil))
}
