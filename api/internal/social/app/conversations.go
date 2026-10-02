package app

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// ConversationRepository persists Conversations, their members and messages, and resolves Mentions.
type ConversationRepository interface {
	InsertConversation(ctx context.Context, id uuid.UUID, title string, by domain.AccountID, members []domain.AccountID, now time.Time) error
	DirectConversation(ctx context.Context, x, y domain.AccountID) (uuid.UUID, error)
	IsMember(ctx context.Context, conversation uuid.UUID, account domain.AccountID) (bool, error)
	Conversations(ctx context.Context, me domain.AccountID) ([]domain.Conversation, error)
	InsertMessage(ctx context.Context, conversation uuid.UUID, m domain.Message, mentions []domain.Mention) error
	Messages(ctx context.Context, conversation uuid.UUID, before time.Time, limit int) ([]domain.Message, []domain.Placed, error)
	MarkRead(ctx context.Context, conversation uuid.UUID, account domain.AccountID, now time.Time) error
	Resolve(ctx context.Context, reader domain.AccountID, m domain.Mention) (domain.Resolved, error)
	Mentionable(ctx context.Context, reader domain.AccountID, q string) ([]domain.Mentionable, error)
}

// Conversation limits.
const (
	MaxMembers  = 10
	MaxMentions = 10
	PageSize    = 50
)

// StartConversation opens a Conversation with Friends: one-to-one, which is found again if it exists,
// or a group with a title.
func (s *Service) StartConversation(ctx context.Context, subject, title string, with []domain.AccountID) (uuid.UUID, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return uuid.Nil, err
	}
	title = strings.TrimSpace(title)
	others := slices.Clone(with)
	slices.SortFunc(others, func(a, b domain.AccountID) int { return strings.Compare(a.String(), b.String()) })
	others = slices.Compact(others)
	if len(others) == 0 || len(others)+1 > MaxMembers || slices.Contains(others, me.ID) || utf8.RuneCountInString(title) > 80 {
		return uuid.Nil, domain.ErrInvalid
	}
	for _, o := range others {
		friends, err := s.Repo.AreFriends(ctx, me.ID, o)
		if err != nil {
			return uuid.Nil, err
		}
		if !friends {
			return uuid.Nil, domain.ErrNotFound
		}
	}
	if len(others) == 1 && title == "" {
		if id, err := s.Repo.DirectConversation(ctx, me.ID, others[0]); err == nil {
			return id, nil
		}
	}
	id := uuid.New()
	return id, s.Repo.InsertConversation(ctx, id, title, me.ID, append([]domain.AccountID{me.ID}, others...), s.Now())
}

// Conversations lists the caller's Conversations, newest first, with unread counts.
func (s *Service) Conversations(ctx context.Context, subject string) ([]domain.Conversation, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, err
	}
	return s.Repo.Conversations(ctx, me.ID)
}

// member checks the caller is in a Conversation; anyone else finds none.
func (s *Service) member(ctx context.Context, subject string, conversation uuid.UUID) (domain.Person, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return me, err
	}
	in, err := s.Repo.IsMember(ctx, conversation, me.ID)
	if err != nil {
		return me, err
	}
	if !in {
		return me, domain.ErrNotFound
	}
	return me, nil
}

// Send posts a message. Each Mention must be something the sender may open themselves.
func (s *Service) Send(ctx context.Context, subject string, conversation uuid.UUID, body string, mentions []domain.Mention) (domain.Message, error) {
	me, err := s.member(ctx, subject, conversation)
	if err != nil {
		return domain.Message{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 4000 || len(mentions) > MaxMentions {
		return domain.Message{}, domain.ErrInvalid
	}
	resolved := make([]domain.Resolved, 0, len(mentions))
	for _, m := range mentions {
		r, err := s.Repo.Resolve(ctx, me.ID, m)
		if err != nil {
			return domain.Message{}, err
		}
		if !r.Open {
			return domain.Message{}, domain.ErrInvalid
		}
		resolved = append(resolved, r)
	}
	msg := domain.Message{ID: uuid.New(), Author: me, Body: body, At: s.Now(), Mentions: resolved}
	return msg, s.Repo.InsertMessage(ctx, conversation, msg, mentions)
}

// Messages reads a page of a Conversation, newest first, before a moment, and marks it read. Each
// Mention shows only to a reader who may open it.
func (s *Service) Messages(ctx context.Context, subject string, conversation uuid.UUID, before *time.Time) ([]domain.Message, error) {
	me, err := s.member(ctx, subject, conversation)
	if err != nil {
		return nil, err
	}
	now := s.Now()
	until := now.Add(time.Second)
	if before != nil {
		until = *before
	}
	msgs, mentions, err := s.Repo.Messages(ctx, conversation, until, PageSize)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]int, len(msgs))
	for i := range msgs {
		msgs[i].Mentions = []domain.Resolved{}
		byID[msgs[i].ID] = i
	}
	for _, m := range mentions {
		r, err := s.Repo.Resolve(ctx, me.ID, m.Mention)
		if err != nil {
			return nil, err
		}
		if i, ok := byID[m.Message]; ok {
			msgs[i].Mentions = append(msgs[i].Mentions, r)
		}
	}
	if before == nil {
		if err := s.Repo.MarkRead(ctx, conversation, me.ID, now); err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

// Mentionable finds game content the caller may mention: Characters in their Campaigns, Locations in
// Campaigns they run.
func (s *Service) Mentionable(ctx context.Context, subject, q string) ([]domain.Mentionable, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, err
	}
	return s.Repo.Mentionable(ctx, me.ID, strings.TrimSpace(q))
}
