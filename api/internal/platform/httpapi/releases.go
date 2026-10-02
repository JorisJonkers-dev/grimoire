package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// ReleaseService is Release Notes.
type ReleaseService interface {
	DraftRelease(ctx context.Context, by, version string) (domain.ReleaseNote, error)
	Releases(ctx context.Context) ([]domain.ReleaseNote, error)
	EditRelease(ctx context.Context, id uuid.UUID, title, body string) (domain.ReleaseNote, error)
	PublishRelease(ctx context.Context, id uuid.UUID, at *time.Time) (domain.ReleaseNote, error)
	UnseenRelease(ctx context.Context, subject string) (*domain.ReleaseNote, error)
	SeeRelease(ctx context.Context, subject string, id uuid.UUID) error
}

func releaseOut(n domain.ReleaseNote) oas.ReleaseNote {
	out := oas.ReleaseNote{
		ID: oas.ID(n.ID), Version: n.Version, Title: n.Title, Body: n.Body, Status: oas.ReleaseNoteStatusDraft,
		PublishAt: oas.OptDateTime{}, CreatedAt: n.CreatedAt.UTC(), UpdatedAt: n.UpdatedAt.UTC(),
	}
	if n.PublishAt != nil {
		out.Status, out.PublishAt = oas.ReleaseNoteStatusScheduled, oas.NewOptDateTime(n.PublishAt.UTC())
	}
	if n.AnnouncedAt != nil {
		out.Status = oas.ReleaseNoteStatusPublished
	}
	return out
}

func (h *Handler) releaseError(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Already there", "That release already has its Release Note, or this one was announced and cannot change.")
	case errors.Is(err, domain.ErrInvalid):
		return problem(http.StatusUnprocessableEntity, "Invalid", "A Release Note is for a full release such as 1.2.0, with a title of up to 120 characters.")
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, "Not found", "There is no such Release Note.")
	}
	return h.friendError(ctx, op, err)
}

// asAdmin runs an Admin-only Release Note change.
func (h *Handler) asAdmin(ctx context.Context) (string, *oas.ProblemStatusCodeWithHeaders) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return "", unauthorized()
	}
	if !h.Accounts.IsAdmin(ctx, id.Subject) {
		return "", problem(http.StatusForbidden, "Forbidden", "Only an Admin can do that.")
	}
	return id.Subject, nil
}

// ListReleaseNotes lists every Release Note for an Admin.
func (h *Handler) ListReleaseNotes(ctx context.Context) (oas.ListReleaseNotesRes, error) {
	if _, bad := h.asAdmin(ctx); bad != nil {
		return bad, nil
	}
	list, err := h.Releases.Releases(ctx)
	if err != nil {
		return h.releaseError(ctx, "list release notes", err), nil
	}
	out := oas.ReleaseNoteList{Items: make([]oas.ReleaseNote, 0, len(list))}
	for _, n := range list {
		out.Items = append(out.Items, releaseOut(n))
	}
	return &oas.ReleaseNoteListHeaders{Response: out}, nil
}

// DraftReleaseNote starts a release's Release Note.
func (h *Handler) DraftReleaseNote(ctx context.Context, req *oas.ReleaseNoteDraft) (oas.DraftReleaseNoteRes, error) {
	by, bad := h.asAdmin(ctx)
	if bad != nil {
		return bad, nil
	}
	n, err := h.Releases.DraftRelease(ctx, by, req.Version)
	if err != nil {
		return h.releaseError(ctx, "draft release note", err), nil
	}
	return &oas.ReleaseNoteHeaders{Response: releaseOut(n)}, nil
}

// EditReleaseNote changes a Release Note's words.
func (h *Handler) EditReleaseNote(ctx context.Context, req *oas.ReleaseNoteChange, p oas.EditReleaseNoteParams) (oas.EditReleaseNoteRes, error) {
	if _, bad := h.asAdmin(ctx); bad != nil {
		return bad, nil
	}
	n, err := h.Releases.EditRelease(ctx, uuid.UUID(p.NoteId), req.Title, req.Body)
	if err != nil {
		return h.releaseError(ctx, "edit release note", err), nil
	}
	return &oas.ReleaseNoteHeaders{Response: releaseOut(n)}, nil
}

// PublishReleaseNote puts a Release Note live now or later.
func (h *Handler) PublishReleaseNote(ctx context.Context, req *oas.ReleaseNotePublish, p oas.PublishReleaseNoteParams) (oas.PublishReleaseNoteRes, error) {
	if _, bad := h.asAdmin(ctx); bad != nil {
		return bad, nil
	}
	var at *time.Time
	if v, set := req.At.Get(); set {
		at = &v
	}
	n, err := h.Releases.PublishRelease(ctx, uuid.UUID(p.NoteId), at)
	if err != nil {
		return h.releaseError(ctx, "publish release note", err), nil
	}
	return &oas.ReleaseNoteHeaders{Response: releaseOut(n)}, nil
}

// GetUnseenReleaseNote reads the newest live Release Note the caller has not seen.
func (h *Handler) GetUnseenReleaseNote(ctx context.Context) (oas.GetUnseenReleaseNoteRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	n, err := h.Releases.UnseenRelease(ctx, id.Subject)
	if err != nil {
		return h.releaseError(ctx, "unseen release note", err), nil
	}
	out := oas.UnseenReleaseNote{Note: oas.OptReleaseNote{}}
	if n != nil {
		out.Note = oas.NewOptReleaseNote(releaseOut(*n))
	}
	return &oas.UnseenReleaseNoteHeaders{Response: out}, nil
}

// SeeReleaseNote marks a Release Note seen.
func (h *Handler) SeeReleaseNote(ctx context.Context, p oas.SeeReleaseNoteParams) (oas.SeeReleaseNoteRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Releases.SeeRelease(ctx, id.Subject, uuid.UUID(p.NoteId)); err != nil {
		return h.releaseError(ctx, "see release note", err), nil
	}
	return &oas.SeeReleaseNoteNoContent{}, nil
}
