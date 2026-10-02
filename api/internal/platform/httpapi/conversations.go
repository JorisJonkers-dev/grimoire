package httpapi

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// ConversationService is Conversations between Friends.
type ConversationService interface {
	StartConversation(ctx context.Context, subject, title string, with []domain.AccountID) (uuid.UUID, error)
	Conversations(ctx context.Context, subject string) ([]domain.Conversation, error)
	Send(ctx context.Context, subject string, conversation uuid.UUID, body string, mentions []domain.Mention) (domain.Message, error)
	Messages(ctx context.Context, subject string, conversation uuid.UUID, before *time.Time) ([]domain.Message, error)
	Mentionable(ctx context.Context, subject, q string) ([]domain.Mentionable, error)
}

func peopleOut(in []domain.Person) []oas.Person {
	out := make([]oas.Person, 0, len(in))
	for _, p := range in {
		out = append(out, personOut(p))
	}
	return out
}

func messageOut(m domain.Message) oas.MessageEntry {
	out := oas.MessageEntry{ID: oas.ID(m.ID), Author: personOut(m.Author), Body: m.Body, At: m.At.UTC(), Mentions: make([]oas.MentionView, 0, len(m.Mentions))}
	for _, r := range m.Mentions {
		v := oas.MentionView{Kind: oas.MentionKind(r.Kind), CampaignId: oas.ID(r.CampaignID), ID: oas.ID(r.TargetID), Open: r.Open, Label: oas.OptString{}, MapId: oas.OptID{}}
		if r.Open {
			v.Label = oas.NewOptString(r.Label)
			if r.MapID != uuid.Nil {
				v.MapId = oas.NewOptID(oas.ID(r.MapID))
			}
		}
		out.Mentions = append(out.Mentions, v)
	}
	return out
}

// ListConversations lists the caller's Conversations.
func (h *Handler) ListConversations(ctx context.Context) (oas.ListConversationsRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Conversations.Conversations(ctx, id.Subject)
	if err != nil {
		return h.friendError(ctx, "list conversations", err), nil
	}
	out := oas.ConversationList{Items: make([]oas.ConversationEntry, 0, len(list))}
	for _, c := range list {
		out.Items = append(out.Items, oas.ConversationEntry{
			ID: oas.ID(c.ID), Title: c.Title, Members: peopleOut(c.Members), UpdatedAt: c.UpdatedAt.UTC(), Unread: int32(c.Unread), LastBody: c.LastBody, //nolint:gosec // a count
		})
	}
	return &oas.ConversationListHeaders{Response: out}, nil
}

// StartConversation opens a Conversation with Friends.
func (h *Handler) StartConversation(ctx context.Context, req *oas.ConversationStart) (oas.StartConversationRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	with := make([]domain.AccountID, 0, len(req.With))
	for _, w := range req.With {
		with = append(with, uuid.UUID(w))
	}
	cid, err := h.Conversations.StartConversation(ctx, id.Subject, req.Title.Or(""), with)
	if err != nil {
		return h.friendError(ctx, "start conversation", err), nil
	}
	return &oas.ConversationRefHeaders{Response: oas.ConversationRef{ID: oas.ID(cid)}}, nil
}

// ListMessages reads a page of a Conversation.
func (h *Handler) ListMessages(ctx context.Context, p oas.ListMessagesParams) (oas.ListMessagesRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	var before *time.Time
	if b, set := p.Before.Get(); set {
		before = &b
	}
	msgs, err := h.Conversations.Messages(ctx, id.Subject, uuid.UUID(p.ConversationId), before)
	if err != nil {
		return h.friendError(ctx, "list messages", err), nil
	}
	out := oas.MessagePage{Items: make([]oas.MessageEntry, 0, len(msgs))}
	for _, m := range msgs {
		out.Items = append(out.Items, messageOut(m))
	}
	return &oas.MessagePageHeaders{Response: out}, nil
}

// SendMessage posts a message.
func (h *Handler) SendMessage(ctx context.Context, req *oas.MessageSend, p oas.SendMessageParams) (oas.SendMessageRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	mentions := make([]domain.Mention, 0, len(req.Mentions))
	for _, m := range req.Mentions {
		mentions = append(mentions, domain.Mention{Kind: string(m.Kind), CampaignID: uuid.UUID(m.CampaignId), TargetID: uuid.UUID(m.ID)})
	}
	msg, err := h.Conversations.Send(ctx, id.Subject, uuid.UUID(p.ConversationId), req.Body, mentions)
	if err != nil {
		return h.friendError(ctx, "send message", err), nil
	}
	return &oas.MessageEntryHeaders{Response: messageOut(msg)}, nil
}

// ListMentionables finds what the caller may mention.
func (h *Handler) ListMentionables(ctx context.Context, p oas.ListMentionablesParams) (oas.ListMentionablesRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	found, err := h.Conversations.Mentionable(ctx, id.Subject, p.Q.Or(""))
	if err != nil {
		return h.friendError(ctx, "list mentionables", err), nil
	}
	out := oas.MentionableList{Items: make([]oas.Mentionable, 0, len(found))}
	for _, m := range found {
		out.Items = append(out.Items, oas.Mentionable{Kind: oas.MentionKind(m.Kind), ID: oas.ID(m.ID), Name: m.Name, CampaignId: oas.ID(m.CampaignID), CampaignName: m.CampaignName})
	}
	return &oas.MentionableListHeaders{Response: out}, nil
}
