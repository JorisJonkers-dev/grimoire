package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// InsertConversation stores a Conversation and its members.
func (s *Store) InsertConversation(ctx context.Context, id uuid.UUID, title string, by domain.AccountID, members []domain.AccountID, now time.Time) error {
	return s.InTx(ctx, func(r app.Repository) error {
		q := r.(*Store).q
		if err := q.InsertConversation(ctx, queries.InsertConversationParams{ID: id, Title: title, CreatedBy: by, Now: now}); err != nil {
			return err
		}
		for _, m := range members {
			if err := q.AddConversationMember(ctx, queries.AddConversationMemberParams{ConversationID: id, AccountID: m, Now: now}); err != nil {
				return err
			}
		}
		return nil
	})
}

// DirectConversation finds the one-to-one Conversation between two Accounts.
func (s *Store) DirectConversation(ctx context.Context, x, y domain.AccountID) (uuid.UUID, error) {
	id, err := s.q.DirectConversation(ctx, queries.DirectConversationParams{X: x, Y: y})
	return id, notFound(err)
}

// IsMember reports whether an Account is in a Conversation.
func (s *Store) IsMember(ctx context.Context, conversation uuid.UUID, account domain.AccountID) (bool, error) {
	return s.q.IsConversationMember(ctx, queries.IsConversationMemberParams{ConversationID: conversation, AccountID: account})
}

// Conversations lists an Account's Conversations with their members and unread counts.
func (s *Store) Conversations(ctx context.Context, me domain.AccountID) ([]domain.Conversation, error) {
	rows, err := s.q.ListConversations(ctx, me)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Conversation, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	at := map[uuid.UUID]int{}
	for i, r := range rows {
		out = append(out, domain.Conversation{ID: r.ID, Title: r.Title, Members: []domain.Person{}, UpdatedAt: r.UpdatedAt, Unread: int(r.Unread), LastBody: r.LastBody})
		ids = append(ids, r.ID)
		at[r.ID] = i
	}
	members, err := s.q.ConversationMembers(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		c := &out[at[m.ConversationID]]
		c.Members = append(c.Members, domain.Person{ID: m.ID, Username: m.Username, Nickname: m.Nickname})
	}
	return out, nil
}

// InsertMessage stores a message with its Mentions and moves its Conversation to the top.
func (s *Store) InsertMessage(ctx context.Context, conversation uuid.UUID, m domain.Message, mentions []domain.Mention) error {
	return s.InTx(ctx, func(r app.Repository) error {
		q := r.(*Store).q
		if err := q.InsertMessage(ctx, queries.InsertMessageParams{ID: m.ID, ConversationID: conversation, Author: m.Author.ID, Body: m.Body, Now: m.At}); err != nil {
			return err
		}
		for i, mention := range mentions {
			if err := q.InsertMention(ctx, queries.InsertMentionParams{
				MessageID: m.ID, Ordinal: int32(i), Kind: mention.Kind, CampaignID: mention.CampaignID, TargetID: mention.TargetID, //nolint:gosec // at most ten
			}); err != nil {
				return err
			}
		}
		return q.TouchConversation(ctx, queries.TouchConversationParams{Now: m.At, ID: conversation})
	})
}

// Messages reads a page of messages before a moment, newest first, with their Mentions.
func (s *Store) Messages(ctx context.Context, conversation uuid.UUID, before time.Time, limit int) ([]domain.Message, []domain.Placed, error) {
	rows, err := s.q.ListMessages(ctx, queries.ListMessagesParams{ConversationID: conversation, Before: before, Lim: int32(limit)}) //nolint:gosec // a page size
	if err != nil {
		return nil, nil, err
	}
	out := make([]domain.Message, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Message{ID: r.ID, Author: domain.Person{ID: r.Author, Username: r.Username, Nickname: r.Nickname}, Body: r.Body, At: r.CreatedAt, Mentions: nil})
		ids = append(ids, r.ID)
	}
	stored, err := s.q.MessageMentions(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	placed := make([]domain.Placed, 0, len(stored))
	for _, m := range stored {
		placed = append(placed, domain.Placed{Message: m.MessageID, Mention: domain.Mention{Kind: m.Kind, CampaignID: m.CampaignID, TargetID: m.TargetID}})
	}
	return out, placed, nil
}

// MarkRead records that an Account read a Conversation.
func (s *Store) MarkRead(ctx context.Context, conversation uuid.UUID, account domain.AccountID, now time.Time) error {
	return s.q.MarkConversationRead(ctx, queries.MarkConversationReadParams{Now: now, ConversationID: conversation, AccountID: account})
}

// Resolve names a Mention for a reader and says whether they may open it.
func (s *Store) Resolve(ctx context.Context, reader domain.AccountID, m domain.Mention) (domain.Resolved, error) {
	out := domain.Resolved{Mention: m, Label: "", MapID: uuid.Nil, Open: false}
	var err error
	switch m.Kind {
	case domain.MentionCharacter:
		out.Label, err = s.q.MentionedCharacter(ctx, queries.MentionedCharacterParams{TargetID: m.TargetID, CampaignID: m.CampaignID, Reader: reader})
	case domain.MentionLocation:
		var r queries.MentionedLocationRow
		r, err = s.q.MentionedLocation(ctx, queries.MentionedLocationParams{TargetID: m.TargetID, CampaignID: m.CampaignID, Reader: reader})
		out.Label, out.MapID = r.Name, r.MapID
	default:
		return out, domain.ErrInvalid
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	out.Open = err == nil
	return out, err
}

// Mentionable finds Characters and Locations a reader may mention.
func (s *Store) Mentionable(ctx context.Context, reader domain.AccountID, q string) ([]domain.Mentionable, error) {
	chars, err := s.q.MentionableCharacters(ctx, queries.MentionableCharactersParams{Reader: reader, Q: q})
	if err != nil {
		return nil, err
	}
	places, err := s.q.MentionableLocations(ctx, queries.MentionableLocationsParams{Reader: reader, Q: q})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Mentionable, 0, len(chars)+len(places))
	for _, c := range chars {
		out = append(out, domain.Mentionable{Kind: domain.MentionCharacter, ID: c.ID, Name: c.Name, CampaignID: c.CampaignID, CampaignName: c.CampaignName})
	}
	for _, p := range places {
		out = append(out, domain.Mentionable{Kind: domain.MentionLocation, ID: p.ID, Name: p.Name, CampaignID: p.CampaignID, CampaignName: p.CampaignName})
	}
	return out, nil
}
