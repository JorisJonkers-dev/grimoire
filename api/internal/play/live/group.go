package live

import (
	"context"
	"reflect"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

const (
	maxGroupName = 40
	// A party splits into a handful of groups at most: each is a Session of its own, with a runtime.
	maxGroups = 4
)

// split reports whether the party plays in more than one Session.
func (s *state) split() bool {
	return len(s.groups) > 1
}

func (s *state) groupViews() []GroupView {
	if !s.split() {
		return nil
	}
	out := make([]GroupView, 0, len(s.groups))
	for _, g := range s.groups {
		v := GroupView{SessionID: uuid.UUID(g.Session).String(), Number: g.Number, Name: g.Name, Home: g.Home, Here: g.Session == s.session.ID, Table: g.Table, Tokens: []string{}}
		for _, t := range g.Tokens {
			v.Tokens = append(v.Tokens, t.Label)
		}
		out = append(out, v)
	}
	return out
}

// busy is why tokens cannot change groups now, or empty: a group forms and returns between scenes.
func (s *state) busy() string {
	switch {
	case s.combat != nil:
		return "Finish the fight first."
	case s.rest != nil || s.cast != nil || len(s.pending) > 0 || len(s.fx.Saves) > 0 || s.sneak != nil || s.explore != nil:
		return "Finish what is under way first: a rest, rolls that are out, sneaking or exploring in turns."
	}
	return ""
}

// leavers are the party tokens a split sends off, or why they cannot go.
func (r *runtime) leavers(ids []string) ([]domain.Token, string) {
	if len(ids) == 0 {
		return nil, "Choose who goes."
	}
	var out []domain.Token
	for _, raw := range ids {
		t, ok := r.st.tokenByID(raw)
		switch {
		case !ok:
			return nil, "No such token."
		case t.Kind != domain.TokenParty:
			return nil, "Only party tokens split off as a group."
		case slices.ContainsFunc(out, func(o domain.Token) bool { return o.ID == t.ID }):
			return nil, "Choose each token once."
		}
		out = append(out, t)
	}
	return out, ""
}

// places puts tokens on the free hexes of a board nearest a hex, or says why it cannot.
func places(on *state, at hex.Coord, tokens []domain.TokenID) ([]domain.TokenPlace, string) {
	if !on.onBoard(at) {
		return nil, "That hex is off the map."
	}
	free := on.freeHexes(at, len(tokens))
	if len(free) < len(tokens) {
		return nil, "There is no room for the group there."
	}
	out := make([]domain.TokenPlace, 0, len(tokens))
	for i, id := range tokens {
		out = append(out, domain.TokenPlace{ID: id, Q: free[i].Q, R: free[i].R})
	}
	return out, ""
}

// split sends party tokens off as a group to a Session of their own on another map. The Players who
// went are sent there after them; nobody who stayed sees where they went.
func (r *runtime) split(req request) {
	name := strings.TrimSpace(req.cmd.Name)
	mapID, err := uuid.Parse(req.cmd.MapID)
	going, reason := r.leavers(req.cmd.TokenIDs)
	switch {
	case r.st.session.Parent != nil:
		reason = "A group that left the party does not split again."
	case len(r.st.groups) >= maxGroups:
		reason = "The party is split into as many groups as it can be."
	case name == "" || wireLen(name) > maxGroupName:
		reason = "Name the group, in up to 40 characters."
	case reason == "" && r.st.busy() != "":
		reason = r.st.busy()
	case reason == "" && err != nil:
		reason = "Choose the map the group goes to."
	}
	if reason != "" {
		r.reject(req, reason)
		return
	}
	board, err := r.store.LoadMap(context.Background(), r.st.session.CampaignID, domain.MapID(mapID))
	if err != nil {
		r.reject(req, "No such map.")
		return
	}
	there := &state{session: r.st.session, tokens: map[domain.TokenID]domain.Token{}}
	there.setBoard(board)
	ids := make([]domain.TokenID, 0, len(going))
	for _, t := range going {
		ids = append(ids, t.ID)
	}
	where, reason := places(there, hex.Coord{Q: req.cmd.Q, R: req.cmd.R}, ids)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	var st *state
	_, done, err := r.store.SplitParty(context.Background(), r.st.session, name, domain.MapID(mapID), where, req.from.Member, req.from.Caller, r.now(), func(tx Store) error {
		var err error
		st, err = r.reload(context.Background(), tx, r.st.session.ID)
		return err
	})
	if err != nil {
		r.log.Error("live: split party", "error", err)
		r.reject(req, "The party could not be split.")
		return
	}
	r.regrouped(req, st, done)
}

// rejoin brings a group back to the Session the party split from, around a hex, and ends its Session.
func (r *runtime) rejoin(req request) {
	id, err := uuid.Parse(req.cmd.SessionID)
	group := domain.SessionID(id)
	at := slices.IndexFunc(r.st.groups, func(g domain.PartyGroup) bool { return g.Session == group && !g.Home })
	switch {
	case r.st.session.Parent != nil:
		r.reject(req, "Bring a group back from the Session the party split from.")
		return
	case err != nil || at < 0:
		r.reject(req, "No such group.")
		return
	case r.st.busy() != "":
		r.reject(req, r.st.busy())
		return
	}
	// The group has to be between scenes too: its Session is read as it is kept.
	away, err := r.reload(context.Background(), r.store, group)
	if err != nil {
		r.log.Error("live: read the group", "error", err)
		r.reject(req, "The group could not be read.")
		return
	}
	if reason := away.busy(); reason != "" {
		r.reject(req, "The group is not ready: "+strings.ToLower(reason[:1])+reason[1:])
		return
	}
	ids := make([]domain.TokenID, 0, len(r.st.groups[at].Tokens))
	for _, t := range r.st.groups[at].Tokens {
		ids = append(ids, t.ID)
	}
	where, reason := places(r.st, hex.Coord{Q: req.cmd.Q, R: req.cmd.R}, ids)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	var st *state
	done, err := r.store.RejoinParty(context.Background(), r.st.session, group, where, req.from.Member, req.from.Caller, r.now(), func(tx Store) error {
		var err error
		st, err = r.reload(context.Background(), tx, r.st.session.ID)
		return err
	})
	if err != nil {
		r.log.Error("live: rejoin party", "error", err)
		r.reject(req, "The group could not be brought back.")
		return
	}
	r.shut(group)
	r.regrouped(req, st, done)
}

// followTable has the Table Display follow one of the groups: this Session's own when none is named.
func (r *runtime) followTable(req request) {
	var group *domain.SessionID
	if req.cmd.SessionID != "" {
		id, err := uuid.Parse(req.cmd.SessionID)
		g := domain.SessionID(id)
		if err != nil || !slices.ContainsFunc(r.st.groups, func(o domain.PartyGroup) bool { return o.Session == g && !o.Home }) {
			r.reject(req, "No such group.")
			return
		}
		group = &g
	}
	if r.st.session.Parent != nil {
		r.reject(req, "Choose what the Table Display follows from the Session the party split from.")
		return
	}
	done, err := r.store.FollowTable(context.Background(), r.st.session, group, req.from.Member, req.from.Caller, r.now())
	if err != nil {
		r.log.Error("live: follow table", "error", err)
		r.reject(req, "That change could not be saved.")
		return
	}
	next := r.st.clone()
	next.session.Seq, next.session.Table = done.Seq, group
	next.groups = slices.Clone(r.st.groups)
	for i, g := range next.groups {
		next.groups[i].Table = (group == nil && g.Home) || (group != nil && g.Session == *group)
	}
	r.regrouped(req, next, done)
}

// regrouped takes the Session as it is after the groups changed: every screen that still belongs here
// is shown it afresh, the others are sent after their group, and the other groups are told.
func (r *runtime) regrouped(req request, st *state, done Committed) {
	r.st, r.lastRoll, r.armed = st, nil, uuid.Nil
	r.sendAway()
	for sub := range r.subs {
		u := r.snapshot(sub.Audience)
		if sub == req.from {
			u.Nonce, u.ActionSeq = req.cmd.Nonce, done.Action
		}
		r.send(sub, u)
	}
	r.arm()
	r.tell(cmdRegroup)
}

// admit lets a screen stay when it belongs with this group. One that belongs with another is sent
// there and let go; one that belongs nowhere it asked for, or whose place cannot be read, is let go
// without a word. While the party is together in the Session it would split from, everyone belongs
// here and nobody is asked.
func (r *runtime) admit(sub *Subscriber) bool {
	if r.st.session.Parent == nil && !r.st.split() {
		return true
	}
	at, err := r.store.Place(context.Background(), r.st.session.ID, sub.Member, sub.Audience)
	if err == nil && at == r.st.session.ID {
		return true
	}
	if err == nil {
		r.send(sub, Update{Kind: UpdRegroup, Seq: r.st.session.Seq, Group: &GroupView{SessionID: uuid.UUID(at).String(), Tokens: []string{}}})
	}
	r.drop(sub)
	return false
}

// sendAway lets go of every screen that no longer belongs with this group.
func (r *runtime) sendAway() {
	for sub := range r.subs {
		r.admit(sub)
	}
}

// partyTokens names the party tokens here and who plays each: what decides who belongs with the group.
func (s *state) partyTokens() string {
	var out []string
	for id, t := range s.tokens {
		if t.Kind != domain.TokenParty {
			continue
		}
		who := ""
		if t.Controller != nil {
			who = t.Controller.String()
		}
		out = append(out, uuid.UUID(id).String()+":"+who+":"+t.Label)
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// replace reads the groups afresh after this group's party tokens changed, and lets go of whoever
// no longer belongs here.
func (r *runtime) replace() {
	groups, err := r.store.Groups(context.Background(), r.st.session)
	if err != nil {
		r.log.Error("live: read groups", "error", err)
	} else {
		r.st.groups = groups
	}
	r.sendAway()
}

// regroup hears from another group that who belongs where has changed. A word that changes nothing
// here, as one that was already heard, is let pass: nobody is asked, and nobody is shown anything.
func (r *runtime) regroup() {
	groups, err := r.store.Groups(context.Background(), r.st.session)
	if err != nil {
		r.log.Error("live: read groups", "error", err)
		return
	}
	if reflect.DeepEqual(groups, r.st.groups) {
		return
	}
	next := r.st.clone()
	next.groups = groups
	r.st = next
	r.sendAway()
	r.show()
}

// refresh hears from another group that the Campaign's Containers, open Shop or day have changed.
func (r *runtime) refresh() {
	trade, err := loadTrade(context.Background(), r.store, r.st.session)
	if err != nil {
		r.log.Error("live: refresh shared state", "error", err)
		return
	}
	next := r.st.clone()
	next.inventory, next.shop, next.day = trade.inventory, trade.shop, trade.day
	r.st = next
	r.show()
}

// show sends every screen the Session as it stands, when nothing of its own changed.
func (r *runtime) show() {
	for sub := range r.subs {
		v := r.st.project(sub.Audience)
		r.send(sub, Update{Kind: UpdView, Seq: r.st.session.Seq, View: &v})
	}
}

// shared reports whether a change touched what every group of the party shares.
func shared(w *Write) bool {
	return w.Move != nil || len(w.Moves) > 0 || w.Drop != nil || w.Gone != nil || w.Claim != nil || w.Trade != nil || len(w.Trades) > 0 ||
		w.Shop != nil || w.Restock != nil || w.Day != nil || len(w.Supplies) > 0 || len(w.Recharged) > 0 || len(w.Results) > 0
}
