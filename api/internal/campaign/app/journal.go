package app

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// JournalRepository keeps a Campaign's Quests and Lore, and knows what each Member's Characters carry.
type JournalRepository interface {
	Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error)
	Quests(ctx context.Context, id domain.CampaignID) ([]domain.Quest, error)
	InsertQuest(ctx context.Context, q domain.Quest, now time.Time) error
	// UpdateQuest and DeleteQuest report ErrNotFound for a Quest the Campaign does not have.
	UpdateQuest(ctx context.Context, q domain.Quest, now time.Time) error
	DeleteQuest(ctx context.Context, id domain.CampaignID, quest domain.QuestID) error
	Lore(ctx context.Context, id domain.CampaignID) ([]domain.Lore, error)
	InsertLore(ctx context.Context, l domain.Lore, now time.Time) error
	// UpdateLore and DeleteLore report ErrNotFound for an entry the Campaign does not have.
	UpdateLore(ctx context.Context, l domain.Lore, now time.Time) error
	DeleteLore(ctx context.Context, id domain.CampaignID, lore domain.LoreID) error
	// CarriedItems lists the items a Member can read: those their own Characters carry and those in
	// the Party Stash. UnlockLore unlocks every locked entry an item holds, and says how many it did.
	CarriedItems(ctx context.Context, id domain.CampaignID, member domain.MemberID) ([]string, error)
	UnlockLore(ctx context.Context, id domain.CampaignID, itemSlug string, now time.Time) (int, error)
}

// Journal runs the Quest and Lore use cases. The DM keeps both. Players see the Quests the party has
// been given and the Lore it has unlocked, and unlock Lore by reading what they carry.
type Journal struct {
	Repo JournalRepository
	Now  func() time.Time
}

// JournalView is the Journal as one Member may see it. Readable lists the items the Member carries
// that still hold Lore; a Player never learns of a locked entry any other way.
type JournalView struct {
	DM       bool
	Quests   []domain.Quest
	Lore     []domain.Lore
	Readable []string
}

func (s *Journal) dm(ctx context.Context, c caller.Caller, id domain.CampaignID) error {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return err
	}
	if me.Role != domain.RoleDM {
		return domain.ErrForbidden
	}
	return nil
}

// Read shows a Member the Journal. A Player gets no hidden Quest and no locked Lore.
func (s *Journal) Read(ctx context.Context, c caller.Caller, id domain.CampaignID) (JournalView, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return JournalView{}, err
	}
	quests, err := s.Repo.Quests(ctx, id)
	if err != nil {
		return JournalView{}, err
	}
	lore, err := s.Repo.Lore(ctx, id)
	if err != nil {
		return JournalView{}, err
	}
	carried, err := s.Repo.CarriedItems(ctx, id, me.ID)
	if err != nil {
		return JournalView{}, err
	}
	v := JournalView{DM: me.Role == domain.RoleDM, Quests: quests, Lore: lore, Readable: []string{}}
	for _, l := range lore {
		if l.UnlockedAt == nil && slices.Contains(carried, l.ItemSlug) && !slices.Contains(v.Readable, l.ItemSlug) {
			v.Readable = append(v.Readable, l.ItemSlug)
		}
	}
	if !v.DM {
		v.Quests = slices.DeleteFunc(slices.Clone(quests), func(q domain.Quest) bool { return q.Status == domain.QuestHidden })
		v.Lore = slices.DeleteFunc(slices.Clone(lore), func(l domain.Lore) bool { return l.UnlockedAt == nil })
		for i := range v.Lore {
			v.Lore[i].ItemSlug = ""
		}
	}
	return v, nil
}

// QuestInput is the editable part of a Quest.
type QuestInput struct {
	Name    string
	Summary string
	Status  string
	Steps   []domain.QuestStep
}

func (s *Journal) quest(ctx context.Context, c caller.Caller, id domain.CampaignID, in QuestInput) (domain.Quest, error) {
	if err := s.dm(ctx, c, id); err != nil {
		return domain.Quest{}, err
	}
	name := strings.TrimSpace(in.Name)
	statuses := []string{domain.QuestHidden, domain.QuestActive, domain.QuestCompleted, domain.QuestFailed}
	if name == "" || utf8.RuneCountInString(name) > 120 || utf8.RuneCountInString(in.Summary) > 4000 || !slices.Contains(statuses, in.Status) || len(in.Steps) > 50 {
		return domain.Quest{}, domain.ErrInvalid
	}
	steps := make([]domain.QuestStep, 0, len(in.Steps))
	for _, step := range in.Steps {
		text := strings.TrimSpace(step.Text)
		if text == "" || utf8.RuneCountInString(text) > 400 {
			return domain.Quest{}, domain.ErrInvalid
		}
		steps = append(steps, domain.QuestStep{Text: text, Done: step.Done})
	}
	return domain.Quest{ID: uuid.Nil, CampaignID: id, Name: name, Summary: in.Summary, Status: in.Status, Steps: steps, UpdatedAt: s.Now()}, nil
}

// CreateQuest adds a Quest. DM only.
func (s *Journal) CreateQuest(ctx context.Context, c caller.Caller, id domain.CampaignID, in QuestInput) (domain.Quest, error) {
	q, err := s.quest(ctx, c, id, in)
	if err != nil {
		return domain.Quest{}, err
	}
	q.ID = uuid.New()
	return q, s.Repo.InsertQuest(ctx, q, q.UpdatedAt)
}

// UpdateQuest changes a Quest: its words, its status, and which steps are done. DM only.
func (s *Journal) UpdateQuest(ctx context.Context, c caller.Caller, id domain.CampaignID, quest domain.QuestID, in QuestInput) (domain.Quest, error) {
	q, err := s.quest(ctx, c, id, in)
	if err != nil {
		return domain.Quest{}, err
	}
	q.ID = quest
	return q, s.Repo.UpdateQuest(ctx, q, q.UpdatedAt)
}

// DeleteQuest removes a Quest. DM only.
func (s *Journal) DeleteQuest(ctx context.Context, c caller.Caller, id domain.CampaignID, quest domain.QuestID) error {
	if err := s.dm(ctx, c, id); err != nil {
		return err
	}
	return s.Repo.DeleteQuest(ctx, id, quest)
}

// LoreInput is the editable part of a Lore entry. Unlocked is the DM's own word that the party knows it.
type LoreInput struct {
	Title    string
	Body     string
	ItemSlug string
	Unlocked bool
}

func (s *Journal) lore(ctx context.Context, c caller.Caller, id domain.CampaignID, in LoreInput) (domain.Lore, error) {
	if err := s.dm(ctx, c, id); err != nil {
		return domain.Lore{}, err
	}
	title, item := strings.TrimSpace(in.Title), strings.TrimSpace(in.ItemSlug)
	if title == "" || utf8.RuneCountInString(title) > 120 || utf8.RuneCountInString(in.Body) > 8000 || len(item) > 80 {
		return domain.Lore{}, domain.ErrInvalid
	}
	l := domain.Lore{ID: uuid.Nil, CampaignID: id, Title: title, Body: in.Body, ItemSlug: item, UnlockedAt: nil, UpdatedAt: s.Now()}
	if in.Unlocked {
		l.UnlockedAt = &l.UpdatedAt
	}
	return l, nil
}

// CreateLore adds a Lore entry, locked unless the DM says the party knows it. DM only.
func (s *Journal) CreateLore(ctx context.Context, c caller.Caller, id domain.CampaignID, in LoreInput) (domain.Lore, error) {
	l, err := s.lore(ctx, c, id, in)
	if err != nil {
		return domain.Lore{}, err
	}
	l.ID = uuid.New()
	return l, s.Repo.InsertLore(ctx, l, l.UpdatedAt)
}

// UpdateLore changes a Lore entry, and locks or unlocks it at the DM's word. DM only.
func (s *Journal) UpdateLore(ctx context.Context, c caller.Caller, id domain.CampaignID, lore domain.LoreID, in LoreInput) (domain.Lore, error) {
	l, err := s.lore(ctx, c, id, in)
	if err != nil {
		return domain.Lore{}, err
	}
	l.ID = lore
	return l, s.Repo.UpdateLore(ctx, l, l.UpdatedAt)
}

// DeleteLore removes a Lore entry. DM only.
func (s *Journal) DeleteLore(ctx context.Context, c caller.Caller, id domain.CampaignID, lore domain.LoreID) error {
	if err := s.dm(ctx, c, id); err != nil {
		return err
	}
	return s.Repo.DeleteLore(ctx, id, lore)
}

// ReadItem is a Member reading a book or letter: it unlocks, for the whole party, every Lore entry
// that item holds. The Member must carry the item, on one of their own Characters or in the Party
// Stash; reading something they do not carry, or that holds nothing, is refused the same way.
func (s *Journal) ReadItem(ctx context.Context, c caller.Caller, id domain.CampaignID, itemSlug string) (int, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return 0, err
	}
	carried, err := s.Repo.CarriedItems(ctx, id, me.ID)
	if err != nil {
		return 0, err
	}
	nothing := refuse("there is nothing to read there")
	if !slices.Contains(carried, itemSlug) {
		return 0, nothing
	}
	n, err := s.Repo.UnlockLore(ctx, id, itemSlug, s.Now())
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nothing
	}
	return n, nil
}
