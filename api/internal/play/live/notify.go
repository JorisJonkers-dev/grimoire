package live

import (
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// Notice is what a member's devices show while the app is not open: a title, a line, and where to go.
type Notice struct {
	Title string
	Body  string
	URL   string
}

// Notifier reaches a member's devices; it must not block the runtime.
type Notifier interface {
	Notify(campaign, member uuid.UUID, n Notice)
}

// nudge tells players when their turn starts or a Reaction Prompt waits for them.
func (r *runtime) nudge(prev, next *state) {
	if r.notify == nil {
		return
	}
	url := "/campaigns/" + r.campaign.String() + "/sessions/" + uuid.UUID(next.session.ID).String()
	was := prev.acting()
	for id := range next.acting() {
		if t := next.tokens[id]; !was[id] && t.Controller != nil {
			r.notify.Notify(r.campaign, *t.Controller, Notice{Title: "Your turn", Body: t.Label + " is up.", URL: url})
		}
	}
	p := prompt(next)
	if p == nil || (prompt(prev) != nil && prompt(prev).ID == p.ID) {
		return
	}
	if t := next.tokens[p.Reactor]; t.Controller != nil {
		r.notify.Notify(r.campaign, *t.Controller, Notice{Title: "Reaction", Body: p.Effect, URL: url})
	}
}

func prompt(s *state) *domain.ReactionPrompt {
	if s.combat == nil {
		return nil
	}
	return s.combat.Prompt
}
