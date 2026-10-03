package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/downtime"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Limits on downtime.
const (
	MaxRecipes       = 200
	MaxDowntimeGrant = 3650
)

// DowntimeStore keeps a Campaign's downtime and the Inventories it draws on.
type DowntimeStore interface {
	LiveSession(ctx context.Context, campaign uuid.UUID) (bool, error)
	LoadInventory(ctx context.Context, campaign uuid.UUID) (domain.Inventory, error)
	Downtime(ctx context.Context, campaign uuid.UUID) (domain.Downtime, error)
	// TrainingDays is how many days a Character has spent training in one thing.
	TrainingDays(ctx context.Context, character uuid.UUID, subject string) (int, error)
	// WriteDowntime runs downtime changes in one transaction.
	WriteDowntime(ctx context.Context, fn func(DowntimeWriter) error) error
}

// DowntimeWriter changes downtime inside a transaction.
type DowntimeWriter interface {
	SetStack(ctx context.Context, in domain.ContainerID, slug string, n int) error
	SetPurse(ctx context.Context, in domain.ContainerID, before, after map[string]int) error
	SetDowntimeDays(ctx context.Context, character uuid.UUID, days, spent int) error
	// GrantDowntime gives days to one Character, or to every Character of the Campaign and starts a new
	// downtime; it reports false for a Character the Campaign does not have.
	GrantDowntime(ctx context.Context, campaign uuid.UUID, character *uuid.UUID, days int) (bool, error)
	MoveDowntimeClock(ctx context.Context, campaign uuid.UUID, to clock.Time, advanced int) error
	LogDowntime(ctx context.Context, e domain.DowntimeEntry) error
	InsertRecipe(ctx context.Context, campaign uuid.UUID, r domain.CampaignRecipe, now time.Time) error
	// DeleteRecipe reports false for a Recipe the Campaign does not have.
	DeleteRecipe(ctx context.Context, campaign, recipe uuid.UUID) (bool, error)
}

// Downtimes runs the downtime use cases: the DM gives downtime days and keeps the Recipes, and a
// Character's Player spends its days on crafting, work, training and research, between Sessions.
type Downtimes struct {
	Store   DowntimeStore
	Members Members
	Now     func() time.Time
}

// DowntimeView is a Campaign's downtime as the caller may see it; Mine marks the Characters the
// caller may spend days for.
type DowntimeView struct {
	DM       bool
	Downtime domain.Downtime
	Mine     []uuid.UUID
}

func (s *Downtimes) view(ctx context.Context, campaign uuid.UUID, me domain.Member) (DowntimeView, error) {
	d, err := s.Store.Downtime(ctx, campaign)
	if err != nil {
		return DowntimeView{}, err
	}
	v := DowntimeView{DM: me.DM, Downtime: d, Mine: []uuid.UUID{}}
	for _, ch := range d.Characters {
		if me.DM || ch.Owner == me.ID {
			v.Mine = append(v.Mine, ch.ID)
		}
	}
	return v, nil
}

// View shows a Member the Campaign's downtime.
func (s *Downtimes) View(ctx context.Context, c caller.Caller, campaign uuid.UUID) (DowntimeView, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return DowntimeView{}, err
	}
	return s.view(ctx, campaign, me)
}

func (s *Downtimes) dm(ctx context.Context, c caller.Caller, campaign uuid.UUID) (domain.Member, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err == nil && !me.DM {
		err = apperr.ErrForbidden
	}
	return me, err
}

// Grant gives downtime days: to one Character, or to every Character of the Campaign, which starts a
// new downtime for the Game Clock to move through. DM only.
func (s *Downtimes) Grant(ctx context.Context, c caller.Caller, campaign uuid.UUID, character *uuid.UUID, days int) (DowntimeView, error) {
	me, err := s.dm(ctx, c, campaign)
	if err != nil {
		return DowntimeView{}, err
	}
	if days < 1 || days > MaxDowntimeGrant {
		return DowntimeView{}, apperr.Refuse("give 1 to 3650 downtime days at a time")
	}
	err = s.Store.WriteDowntime(ctx, func(w DowntimeWriter) error {
		found, err := w.GrantDowntime(ctx, campaign, character, days)
		if err == nil && !found {
			err = apperr.ErrNotFound
		}
		return err
	})
	if err != nil {
		return DowntimeView{}, err
	}
	return s.view(ctx, campaign, me)
}

// AddRecipe adds a Recipe to the Campaign. DM only.
func (s *Downtimes) AddRecipe(ctx context.Context, c caller.Caller, campaign uuid.UUID, r downtime.Recipe) (domain.CampaignRecipe, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return domain.CampaignRecipe{}, err
	}
	r.Name = strings.TrimSpace(r.Name)
	if err := downtime.CheckRecipe(r); err != nil {
		return domain.CampaignRecipe{}, apperr.Refuse(err.Error())
	}
	d, err := s.Store.Downtime(ctx, campaign)
	if err != nil {
		return domain.CampaignRecipe{}, err
	}
	if len(d.Recipes) >= MaxRecipes {
		return domain.CampaignRecipe{}, apperr.Refuse("a Campaign keeps up to 200 Recipes")
	}
	made := domain.CampaignRecipe{ID: uuid.New(), Recipe: r}
	return made, s.Store.WriteDowntime(ctx, func(w DowntimeWriter) error { return w.InsertRecipe(ctx, campaign, made, s.Now()) })
}

// RemoveRecipe removes a Recipe. DM only.
func (s *Downtimes) RemoveRecipe(ctx context.Context, c caller.Caller, campaign, recipe uuid.UUID) error {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return err
	}
	return s.Store.WriteDowntime(ctx, func(w DowntimeWriter) error {
		found, err := w.DeleteRecipe(ctx, campaign, recipe)
		if err == nil && !found {
			err = apperr.ErrNotFound
		}
		return err
	})
}

// DowntimeActivity is how a Character spends downtime days: crafting from a Recipe, which takes the
// days the Recipe says, or working, training in a subject or researching one for a number of days.
type DowntimeActivity struct {
	Kind    string
	Days    int
	Recipe  uuid.UUID
	Subject string
}

// spending is a Character about to spend downtime: its days, its own Container and what it holds.
type spending struct {
	campaign uuid.UUID
	who      domain.DowntimeCharacter
	mine     domain.Container
	down     domain.Downtime
}

// craft makes a Recipe from what a Character holds. Ingredients come off its stacks. Its tool may be
// an Item Instance instead, such as a named kit: it is then at hand without being a stack.
func craft(sp spending, r downtime.Recipe) (downtime.Crafted, error) {
	if slices.ContainsFunc(sp.mine.Instances, func(in domain.Instance) bool { return in.Slug == r.Tool }) {
		r.Tool = ""
	}
	return downtime.Craft(r, downtime.Bench{Days: sp.who.Days, Coins: sp.mine.Coins, Items: sp.mine.Items})
}

// Spend has a Character spend downtime days, between Sessions. Crafting takes the Recipe's ingredients
// from the Character's own Inventory and puts what it makes there. The Game Clock moves on by the days
// nobody had yet lived through. The Character's Player or the DM.
func (s *Downtimes) Spend(ctx context.Context, c caller.Caller, campaign, character uuid.UUID, a DowntimeActivity) (DowntimeView, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return DowntimeView{}, err
	}
	live, err := s.Store.LiveSession(ctx, campaign)
	if err != nil {
		return DowntimeView{}, err
	}
	if live {
		return DowntimeView{}, apperr.Refuse("a Session is live: downtime is spent between Sessions")
	}
	d, err := s.Store.Downtime(ctx, campaign)
	if err != nil {
		return DowntimeView{}, err
	}
	i := slices.IndexFunc(d.Characters, func(ch domain.DowntimeCharacter) bool { return ch.ID == character })
	if i < 0 {
		return DowntimeView{}, apperr.ErrNotFound
	}
	if d.Characters[i].Owner != me.ID && !me.DM {
		return DowntimeView{}, apperr.ErrForbidden
	}
	inv, err := s.Store.LoadInventory(ctx, campaign)
	if err != nil {
		return DowntimeView{}, err
	}
	sp := spending{campaign: campaign, who: d.Characters[i], mine: characterContainer(inv, character), down: d}
	change, err := s.plan(ctx, sp, a)
	if err != nil {
		return DowntimeView{}, err
	}
	if err := s.Store.WriteDowntime(ctx, change); err != nil {
		return DowntimeView{}, err
	}
	return s.view(ctx, campaign, me)
}

// outcome is what an activity leaves: the days and coin left, the Items it changed, the days it took
// and what the log says of it.
type outcome struct {
	daysLeft int
	coins    map[string]int
	items    map[string]int
	took     int
	detail   string
}

// refused turns a refusal of the rules into one the caller is shown.
func refused(err error) error {
	var no downtime.RefusedError
	if errors.As(err, &no) {
		return apperr.Refuse(string(no))
	}
	return err
}

func (s *Downtimes) outcome(ctx context.Context, sp spending, a DowntimeActivity) (outcome, error) {
	subject := strings.TrimSpace(a.Subject)
	switch a.Kind {
	case domain.DowntimeCraft:
		i := slices.IndexFunc(sp.down.Recipes, func(r domain.CampaignRecipe) bool { return r.ID == a.Recipe })
		if i < 0 {
			return outcome{}, apperr.ErrNotFound
		}
		r := sp.down.Recipes[i].Recipe
		made, err := craft(sp, r)
		return outcome{daysLeft: made.DaysLeft, coins: made.Coins, items: made.Items, took: r.Days, detail: r.Name}, refused(err)
	case domain.DowntimeWork:
		left, _, err := downtime.Spend(sp.who.Days, sp.mine.Coins, a.Days, 0)
		return outcome{daysLeft: left, coins: shops.Coins(shops.Worth(sp.mine.Coins) + downtime.Wage(a.Days)), items: nil, took: a.Days, detail: ""}, refused(err)
	case domain.DowntimeTrain, domain.DowntimeResearch:
		if subject == "" || utf8.RuneCountInString(subject) > 80 {
			return outcome{}, apperr.Refuse("say what the training or research is in, in up to 80 characters")
		}
		cost := downtime.ResearchCost(a.Days)
		if a.Kind == domain.DowntimeTrain {
			cost = downtime.TrainingCost(a.Days)
			done, err := s.Store.TrainingDays(ctx, sp.who.ID, subject)
			if err != nil {
				return outcome{}, err
			}
			if downtime.Trained(done) {
				return outcome{}, apperr.Refuse(fmt.Sprintf("%s has finished training in %s", sp.who.Name, subject))
			}
		}
		left, coins, err := downtime.Spend(sp.who.Days, sp.mine.Coins, a.Days, cost)
		return outcome{daysLeft: left, coins: coins, items: nil, took: a.Days, detail: subject}, refused(err)
	}
	return outcome{}, apperr.Refuse("downtime is spent crafting, working, training or researching")
}

// plan works out what an activity changes and returns the change to write.
func (s *Downtimes) plan(ctx context.Context, sp spending, a DowntimeActivity) (func(DowntimeWriter) error, error) {
	out, err := s.outcome(ctx, sp, a)
	if err != nil {
		return nil, err
	}
	spent := sp.who.Spent + out.took
	onward := downtime.ClockDays(sp.down.Advanced, spent)
	entry := domain.DowntimeEntry{ID: uuid.New(), Campaign: sp.campaign, Character: sp.who.ID, Name: sp.who.Name, Activity: a.Kind, Detail: out.detail, Days: out.took, At: s.Now()}
	return func(w DowntimeWriter) error {
		for slug, n := range out.items {
			if err := w.SetStack(ctx, sp.mine.ID, slug, n); err != nil {
				return err
			}
		}
		if err := w.SetPurse(ctx, sp.mine.ID, sp.mine.Coins, out.coins); err != nil {
			return err
		}
		if err := w.SetDowntimeDays(ctx, sp.who.ID, out.daysLeft, spent); err != nil {
			return err
		}
		if onward > 0 {
			if err := w.MoveDowntimeClock(ctx, sp.campaign, sp.down.Clock.Add(onward*clock.DayMinutes), sp.down.Advanced+onward); err != nil {
				return err
			}
		}
		return w.LogDowntime(ctx, entry)
	}, nil
}
