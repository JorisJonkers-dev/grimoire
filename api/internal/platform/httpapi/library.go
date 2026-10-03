package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"

	"github.com/go-faster/jx"
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// LibraryService is an account's Library and its links into Campaigns.
type LibraryService interface {
	Entries(ctx context.Context, c caller.Caller, kind string) ([]domain.Entry, error)
	Create(ctx context.Context, c caller.Caller, d domain.Draft) (domain.Entry, error)
	Get(ctx context.Context, c caller.Caller, id uuid.UUID) (domain.Detail, error)
	Update(ctx context.Context, c caller.Caller, id uuid.UUID, d domain.Draft) (domain.Detail, error)
	Linked(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Linked, error)
	Link(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID) (domain.Linked, error)
	Unlink(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID) error
	Override(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID, fields domain.Fields) (domain.Linked, error)
	Pin(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID, revision *int) (domain.Linked, error)
	Collections(ctx context.Context, c caller.Caller) ([]domain.Collection, error)
	CreateCollection(ctx context.Context, c caller.Caller, name, description string) (domain.Collection, error)
	UpdateCollection(ctx context.Context, c caller.Caller, id uuid.UUID, name, description string, entries []uuid.UUID) (domain.Collection, error)
	CampaignCollections(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Collection, error)
	Switch(ctx context.Context, c caller.Caller, campaign, collection uuid.UUID, on bool) ([]domain.Collection, error)
	Propose(ctx context.Context, c caller.Caller, campaign uuid.UUID, d domain.Draft, note string, base *uuid.UUID) (domain.Proposal, error)
	Proposals(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Proposal, error)
	Proposal(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (domain.ProposalDetail, error)
	Resubmit(ctx context.Context, c caller.Caller, campaign, id uuid.UUID, d domain.Draft, note string) (domain.ProposalDetail, error)
	Review(ctx context.Context, c caller.Caller, campaign, id uuid.UUID, action, message string, edit *domain.Draft) (domain.ProposalDetail, error)
	Shared(ctx context.Context, kind string) ([]domain.Entry, error)
	Share(ctx context.Context, c caller.Caller, entry uuid.UUID, note string) (domain.Submission, error)
	Submissions(ctx context.Context, c caller.Caller) ([]domain.Submission, error)
	AllSubmissions(ctx context.Context, c caller.Caller) ([]domain.Submission, error)
	ReviewSubmission(ctx context.Context, c caller.Caller, id uuid.UUID, approve, ipClear bool, ipNote, message string) (domain.Submission, error)
	Export(ctx context.Context, c caller.Caller, collection, entry *uuid.UUID) (domain.Export, error)
	Import(ctx context.Context, c caller.Caller, in []domain.Incoming, cols []domain.ExportedCollection) (app.Report, error)
}

var _ LibraryService = (*app.Service)(nil)

// libraryProblem maps a Library error to a problem; anything unexpected is logged and hidden.
func (h *Handler) libraryProblem(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	var rule *apperr.RuleError
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return problem(http.StatusNotFound, "Not found", "No such Library entry, Campaign or link.")
	case errors.Is(err, apperr.ErrForbidden):
		return problem(http.StatusForbidden, "Forbidden", "Only the Campaign's DM can do that.")
	case errors.As(err, &rule):
		return problem(http.StatusUnprocessableEntity, "Not allowed", rule.Reason)
	}
	h.Log.ErrorContext(ctx, op, "error", err)
	return unavailable()
}

func fieldsIn(in oas.LibraryFields) domain.Fields {
	out := domain.Fields{}
	for _, f := range in {
		out[f.Name] = f.Value
	}
	return out
}

func fieldsOut(in domain.Fields) oas.LibraryFields {
	out := oas.LibraryFields{}
	for _, name := range slices.Sorted(maps.Keys(in)) {
		out = append(out, oas.LibraryField{Name: name, Value: in[name]})
	}
	return out
}

func optRevision(n *int) oas.OptInt32 {
	if n == nil {
		return oas.OptInt32{}
	}
	return oas.NewOptInt32(int32(*n)) //nolint:gosec // revision numbers are small
}

func libraryEntryOut(e domain.Entry) oas.LibraryEntry {
	return oas.LibraryEntry{
		ID: oas.ID(e.ID), Kind: oas.LibraryKind(e.Kind), Name: e.Name, Fields: fieldsOut(e.Fields), Revision: int32(e.Revision), //nolint:gosec // revision numbers are small
		CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC(), Shared: optShared(e.Shared),
	}
}

func optShared(shared bool) oas.OptBool {
	if !shared {
		return oas.OptBool{}
	}
	return oas.NewOptBool(true)
}

func libraryDetailOut(d domain.Detail) *oas.LibraryEntryDetailHeaders {
	out := oas.LibraryEntryDetail{Entry: libraryEntryOut(d.Entry), Revisions: []oas.LibraryRevision{}, Uses: []oas.LibraryUse{}}
	for _, r := range d.Revisions {
		out.Revisions = append(out.Revisions, oas.LibraryRevision{No: int32(r.No), Name: r.Name, Fields: fieldsOut(r.Fields), CreatedAt: r.At.UTC()}) //nolint:gosec // revision numbers are small
	}
	for _, u := range d.Uses {
		out.Uses = append(out.Uses, oas.LibraryUse{CampaignId: oas.ID(u.CampaignID), Campaign: u.Campaign, PinnedRevision: optRevision(u.Pinned)})
	}
	return &oas.LibraryEntryDetailHeaders{Response: out}
}

func linkedOut(l domain.Linked) oas.LinkedEntry {
	return oas.LinkedEntry{
		Entry: libraryEntryOut(l.Entry), PinnedRevision: optRevision(l.Pinned), BaseName: l.BaseName,
		Base: fieldsOut(l.Base), Override: fieldsOut(l.Override), Fields: fieldsOut(l.Resolved()), Direct: l.Direct, Via: append([]string{}, l.Via...),
	}
}

func collectionOut(c domain.Collection, me string, campaign bool) oas.LibraryCollection {
	out := oas.LibraryCollection{ID: oas.ID(c.ID), Name: c.Name, Description: c.Description, EntryIds: []oas.ID{}, Mine: c.Owner == me}
	for _, e := range c.Entries {
		out.EntryIds = append(out.EntryIds, oas.ID(e))
	}
	if campaign {
		out.SwitchedOn = oas.NewOptBool(c.On)
	}
	return out
}

func collectionsOut(list []domain.Collection, me string, campaign bool) []oas.LibraryCollection {
	out := make([]oas.LibraryCollection, 0, len(list))
	for _, c := range list {
		out = append(out, collectionOut(c, me, campaign))
	}
	return out
}

// libraryCall runs a Library use case as the caller and maps its error.
func libraryCall[T any](ctx context.Context, h *Handler, op string, run func(c caller.Caller) (T, error)) (T, *oas.ProblemStatusCodeWithHeaders) {
	var zero T
	c, signed := uiCaller(ctx)
	if !signed {
		return zero, unauthorized()
	}
	v, err := run(c)
	if err != nil {
		return zero, h.libraryProblem(ctx, op, err)
	}
	return v, nil
}

// ListLibraryEntries lists the caller's Library.
func (h *Handler) ListLibraryEntries(ctx context.Context, p oas.ListLibraryEntriesParams) (oas.ListLibraryEntriesRes, error) {
	list, bad := libraryCall(ctx, h, "list library", func(c caller.Caller) ([]domain.Entry, error) {
		return h.Library.Entries(ctx, c, string(p.Kind.Or("")))
	})
	if bad != nil {
		return bad, nil
	}
	out := make([]oas.LibraryEntry, 0, len(list))
	for _, e := range list {
		out = append(out, libraryEntryOut(e))
	}
	return &oas.ListLibraryEntriesOKHeaders{Response: out}, nil
}

// CreateLibraryEntry adds an entry to the caller's Library.
func (h *Handler) CreateLibraryEntry(ctx context.Context, req *oas.LibraryEntryInput) (oas.CreateLibraryEntryRes, error) {
	e, bad := libraryCall(ctx, h, "create library entry", func(c caller.Caller) (domain.Entry, error) {
		return h.Library.Create(ctx, c, domain.Draft{Kind: string(req.Kind), Name: req.Name, Fields: fieldsIn(req.Fields)})
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.LibraryEntryHeaders{Response: libraryEntryOut(e)}, nil
}

// GetLibraryEntry reads one of the caller's entries.
func (h *Handler) GetLibraryEntry(ctx context.Context, p oas.GetLibraryEntryParams) (oas.GetLibraryEntryRes, error) {
	d, bad := libraryCall(ctx, h, "get library entry", func(c caller.Caller) (domain.Detail, error) {
		return h.Library.Get(ctx, c, uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return libraryDetailOut(d), nil
}

// UpdateLibraryEntry saves a new base for one of the caller's entries.
func (h *Handler) UpdateLibraryEntry(ctx context.Context, req *oas.LibraryEntryUpdate, p oas.UpdateLibraryEntryParams) (oas.UpdateLibraryEntryRes, error) {
	d, bad := libraryCall(ctx, h, "update library entry", func(c caller.Caller) (domain.Detail, error) {
		return h.Library.Update(ctx, c, uuid.UUID(p.EntryId), domain.Draft{Name: req.Name, Fields: fieldsIn(req.Fields)})
	})
	if bad != nil {
		return bad, nil
	}
	return libraryDetailOut(d), nil
}

// ListLinkedEntries lists what a Campaign links from Libraries.
func (h *Handler) ListLinkedEntries(ctx context.Context, p oas.ListLinkedEntriesParams) (oas.ListLinkedEntriesRes, error) {
	list, bad := libraryCall(ctx, h, "list linked entries", func(c caller.Caller) ([]domain.Linked, error) {
		return h.Library.Linked(ctx, c, uuid.UUID(p.CampaignId))
	})
	if bad != nil {
		return bad, nil
	}
	out := make([]oas.LinkedEntry, 0, len(list))
	for _, l := range list {
		out = append(out, linkedOut(l))
	}
	return &oas.ListLinkedEntriesOKHeaders{Response: out}, nil
}

// linkedCall runs a change to one link and answers with the link as the Campaign now sees it.
func (h *Handler) linkedCall(ctx context.Context, op string, run func(c caller.Caller) (domain.Linked, error)) (*oas.LinkedEntryHeaders, *oas.ProblemStatusCodeWithHeaders) {
	l, bad := libraryCall(ctx, h, op, run)
	if bad != nil {
		return nil, bad
	}
	return &oas.LinkedEntryHeaders{Response: linkedOut(l)}, nil
}

// LinkLibraryEntry links one of the caller's entries into a Campaign.
func (h *Handler) LinkLibraryEntry(ctx context.Context, req *oas.LibraryLinkInput, p oas.LinkLibraryEntryParams) (oas.LinkLibraryEntryRes, error) {
	out, bad := h.linkedCall(ctx, "link library entry", func(c caller.Caller) (domain.Linked, error) {
		return h.Library.Link(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(req.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// UnlinkLibraryEntry takes an entry out of a Campaign.
func (h *Handler) UnlinkLibraryEntry(ctx context.Context, p oas.UnlinkLibraryEntryParams) (oas.UnlinkLibraryEntryRes, error) {
	_, bad := libraryCall(ctx, h, "unlink library entry", func(c caller.Caller) (struct{}, error) {
		return struct{}{}, h.Library.Unlink(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.UnlinkLibraryEntryNoContent{}, nil
}

// SetCampaignOverride replaces a linked entry's Campaign Override.
func (h *Handler) SetCampaignOverride(ctx context.Context, req *oas.CampaignOverrideInput, p oas.SetCampaignOverrideParams) (oas.SetCampaignOverrideRes, error) {
	out, bad := h.linkedCall(ctx, "set campaign override", func(c caller.Caller) (domain.Linked, error) {
		return h.Library.Override(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.EntryId), fieldsIn(req.Fields))
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// PinLibraryRevision holds a Campaign to one Revision of a linked entry.
func (h *Handler) PinLibraryRevision(ctx context.Context, req *oas.LibraryPinInput, p oas.PinLibraryRevisionParams) (oas.PinLibraryRevisionRes, error) {
	out, bad := h.linkedCall(ctx, "pin library revision", func(c caller.Caller) (domain.Linked, error) {
		no := int(req.Revision)
		return h.Library.Pin(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.EntryId), &no)
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// UnpinLibraryRevision makes a Campaign follow a linked entry's latest Revision again.
func (h *Handler) UnpinLibraryRevision(ctx context.Context, p oas.UnpinLibraryRevisionParams) (oas.UnpinLibraryRevisionRes, error) {
	out, bad := h.linkedCall(ctx, "unpin library revision", func(c caller.Caller) (domain.Linked, error) {
		return h.Library.Pin(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.EntryId), nil)
	})
	if bad != nil {
		return bad, nil
	}
	return out, nil
}

// collectionsCall runs a Collection use case as the caller, keeping who they are for the answer.
func collectionsCall[T any](ctx context.Context, h *Handler, op string, run func(c caller.Caller) (T, error)) (T, string, *oas.ProblemStatusCodeWithHeaders) {
	var me string
	v, bad := libraryCall(ctx, h, op, func(c caller.Caller) (T, error) {
		me = c.Subject
		return run(c)
	})
	return v, me, bad
}

// ListLibraryCollections lists the caller's Collections.
func (h *Handler) ListLibraryCollections(ctx context.Context) (oas.ListLibraryCollectionsRes, error) {
	list, me, bad := collectionsCall(ctx, h, "list collections", func(c caller.Caller) ([]domain.Collection, error) {
		return h.Library.Collections(ctx, c)
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.ListLibraryCollectionsOKHeaders{Response: collectionsOut(list, me, false)}, nil
}

// CreateLibraryCollection starts an empty Collection.
func (h *Handler) CreateLibraryCollection(ctx context.Context, req *oas.LibraryCollectionInput) (oas.CreateLibraryCollectionRes, error) {
	col, me, bad := collectionsCall(ctx, h, "create collection", func(c caller.Caller) (domain.Collection, error) {
		return h.Library.CreateCollection(ctx, c, req.Name, req.Description.Or(""))
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.LibraryCollectionHeaders{Response: collectionOut(col, me, false)}, nil
}

// UpdateLibraryCollection renames a Collection and sets its entries.
func (h *Handler) UpdateLibraryCollection(ctx context.Context, req *oas.LibraryCollectionUpdate, p oas.UpdateLibraryCollectionParams) (oas.UpdateLibraryCollectionRes, error) {
	entries := make([]uuid.UUID, 0, len(req.EntryIds))
	for _, e := range req.EntryIds {
		entries = append(entries, uuid.UUID(e))
	}
	col, me, bad := collectionsCall(ctx, h, "update collection", func(c caller.Caller) (domain.Collection, error) {
		return h.Library.UpdateCollection(ctx, c, uuid.UUID(p.CollectionId), req.Name, req.Description.Or(""), entries)
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.LibraryCollectionHeaders{Response: collectionOut(col, me, false)}, nil
}

// ListCampaignCollections lists the Collections a DM can switch in a Campaign.
func (h *Handler) ListCampaignCollections(ctx context.Context, p oas.ListCampaignCollectionsParams) (oas.ListCampaignCollectionsRes, error) {
	list, me, bad := collectionsCall(ctx, h, "list campaign collections", func(c caller.Caller) ([]domain.Collection, error) {
		return h.Library.CampaignCollections(ctx, c, uuid.UUID(p.CampaignId))
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.ListCampaignCollectionsOKHeaders{Response: collectionsOut(list, me, true)}, nil
}

// SwitchLibraryCollection turns a Collection on or off in a Campaign.
func (h *Handler) SwitchLibraryCollection(ctx context.Context, req *oas.LibrarySwitchInput, p oas.SwitchLibraryCollectionParams) (oas.SwitchLibraryCollectionRes, error) {
	list, me, bad := collectionsCall(ctx, h, "switch collection", func(c caller.Caller) ([]domain.Collection, error) {
		return h.Library.Switch(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.CollectionId), req.On)
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.SwitchLibraryCollectionOKHeaders{Response: collectionsOut(list, me, true)}, nil
}

func optID(id *uuid.UUID) oas.OptID {
	if id == nil {
		return oas.OptID{}
	}
	return oas.NewOptID(oas.ID(*id))
}

func proposalOut(p domain.Proposal) oas.Proposal {
	return oas.Proposal{
		ID: oas.ID(p.ID), Kind: oas.LibraryKind(p.Draft.Kind), Name: p.Draft.Name, Fields: fieldsOut(p.Draft.Fields), Note: p.Note,
		AuthorName: p.AuthorName, Status: oas.ProposalStatus(p.Status), Message: p.Message, BaseEntryId: optID(p.Base), EntryId: optID(p.Entry),
		CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.UpdatedAt.UTC(),
	}
}

func proposalDetailOut(d domain.ProposalDetail) *oas.ProposalDetailHeaders {
	out := oas.ProposalDetail{Proposal: proposalOut(d.Proposal), Steps: []oas.ProposalStep{}}
	for _, r := range d.Reviews {
		out.Steps = append(out.Steps, oas.ProposalStep{No: int32(r.No), Action: oas.ProposalStepAction(r.Action), Message: r.Message, By: r.By, CreatedAt: r.At.UTC()}) //nolint:gosec // a short history
	}
	if d.Current != nil {
		out.Current = oas.NewOptLinkedEntry(linkedOut(*d.Current))
	}
	return &oas.ProposalDetailHeaders{Response: out}
}

// ListProposals lists a Campaign's Proposals.
func (h *Handler) ListProposals(ctx context.Context, p oas.ListProposalsParams) (oas.ListProposalsRes, error) {
	list, bad := libraryCall(ctx, h, "list proposals", func(c caller.Caller) ([]domain.Proposal, error) {
		return h.Library.Proposals(ctx, c, uuid.UUID(p.CampaignId))
	})
	if bad != nil {
		return bad, nil
	}
	out := make([]oas.Proposal, 0, len(list))
	for _, x := range list {
		out = append(out, proposalOut(x))
	}
	return &oas.ListProposalsOKHeaders{Response: out}, nil
}

// CreateProposal sends the DM a new entry, or a change to one the Campaign sees.
func (h *Handler) CreateProposal(ctx context.Context, req *oas.ProposalInput, p oas.CreateProposalParams) (oas.CreateProposalRes, error) {
	var base *uuid.UUID
	if id, ok := req.BaseEntryId.Get(); ok {
		b := uuid.UUID(id)
		base = &b
	}
	out, bad := libraryCall(ctx, h, "create proposal", func(c caller.Caller) (domain.Proposal, error) {
		d := domain.Draft{Kind: string(req.Kind), Name: req.Name, Fields: fieldsIn(req.Fields)}
		return h.Library.Propose(ctx, c, uuid.UUID(p.CampaignId), d, req.Note.Or(""), base)
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.ProposalHeaders{Response: proposalOut(out)}, nil
}

// GetProposal reads one Proposal.
func (h *Handler) GetProposal(ctx context.Context, p oas.GetProposalParams) (oas.GetProposalRes, error) {
	d, bad := libraryCall(ctx, h, "get proposal", func(c caller.Caller) (domain.ProposalDetail, error) {
		return h.Library.Proposal(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.ProposalId))
	})
	if bad != nil {
		return bad, nil
	}
	return proposalDetailOut(d), nil
}

// ResubmitProposal sends a Proposal again after the DM asked for changes.
func (h *Handler) ResubmitProposal(ctx context.Context, req *oas.ProposalUpdate, p oas.ResubmitProposalParams) (oas.ResubmitProposalRes, error) {
	d, bad := libraryCall(ctx, h, "resubmit proposal", func(c caller.Caller) (domain.ProposalDetail, error) {
		return h.Library.Resubmit(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.ProposalId), domain.Draft{Name: req.Name, Fields: fieldsIn(req.Fields)}, req.Note.Or(""))
	})
	if bad != nil {
		return bad, nil
	}
	return proposalDetailOut(d), nil
}

// ReviewProposal decides a pending Proposal.
func (h *Handler) ReviewProposal(ctx context.Context, req *oas.ProposalReviewInput, p oas.ReviewProposalParams) (oas.ReviewProposalRes, error) {
	var edit *domain.Draft
	if req.Name.Set || req.Fields != nil {
		edit = &domain.Draft{Name: req.Name.Or("")}
		if req.Fields != nil {
			edit.Fields = fieldsIn(req.Fields)
		}
	}
	d, bad := libraryCall(ctx, h, "review proposal", func(c caller.Caller) (domain.ProposalDetail, error) {
		return h.Library.Review(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.ProposalId), string(req.Action), req.Message.Or(""), edit)
	})
	if bad != nil {
		return bad, nil
	}
	return proposalDetailOut(d), nil
}

func submissionOut(x domain.Submission) oas.SharedSubmission {
	out := oas.SharedSubmission{
		ID: oas.ID(x.ID), EntryId: oas.ID(x.Entry), Revision: int32(x.Revision), Kind: oas.LibraryKind(x.Draft.Kind), Name: x.Draft.Name, //nolint:gosec // revision numbers are small
		Fields: fieldsOut(x.Draft.Fields), Note: x.Note, Status: oas.SharedSubmissionStatus(x.Status), IpNote: x.IPNote, Message: x.Message,
		SharedEntryId: optID(x.Shared), CreatedAt: x.CreatedAt.UTC(),
	}
	if x.IPClear != nil {
		out.IpClear = oas.NewOptBool(*x.IPClear)
	}
	if x.DecidedAt != nil {
		out.DecidedAt = oas.NewOptDateTime(x.DecidedAt.UTC())
	}
	return out
}

func submissionsOut(list []domain.Submission) []oas.SharedSubmission {
	out := make([]oas.SharedSubmission, 0, len(list))
	for _, x := range list {
		out = append(out, submissionOut(x))
	}
	return out
}

// ListSharedEntries lists the Shared Library.
func (h *Handler) ListSharedEntries(ctx context.Context, p oas.ListSharedEntriesParams) (oas.ListSharedEntriesRes, error) {
	list, bad := libraryCall(ctx, h, "list shared", func(caller.Caller) ([]domain.Entry, error) {
		return h.Library.Shared(ctx, string(p.Kind.Or("")))
	})
	if bad != nil {
		return bad, nil
	}
	out := make([]oas.LibraryEntry, 0, len(list))
	for _, e := range list {
		out = append(out, libraryEntryOut(e))
	}
	return &oas.ListSharedEntriesOKHeaders{Response: out}, nil
}

// ListMySubmissions lists the caller's requests to share.
func (h *Handler) ListMySubmissions(ctx context.Context) (oas.ListMySubmissionsRes, error) {
	list, bad := libraryCall(ctx, h, "list my submissions", func(c caller.Caller) ([]domain.Submission, error) {
		return h.Library.Submissions(ctx, c)
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.ListMySubmissionsOKHeaders{Response: submissionsOut(list)}, nil
}

// ShareLibraryEntry asks the Admins to share an entry.
func (h *Handler) ShareLibraryEntry(ctx context.Context, req *oas.ShareInput) (oas.ShareLibraryEntryRes, error) {
	x, bad := libraryCall(ctx, h, "share library entry", func(c caller.Caller) (domain.Submission, error) {
		return h.Library.Share(ctx, c, uuid.UUID(req.EntryId), req.Note.Or(""))
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.SharedSubmissionHeaders{Response: submissionOut(x)}, nil
}

// ListSharedSubmissions lists every request to share, for an Admin.
func (h *Handler) ListSharedSubmissions(ctx context.Context) (oas.ListSharedSubmissionsRes, error) {
	list, bad := libraryCall(ctx, h, "list submissions", func(c caller.Caller) ([]domain.Submission, error) {
		return h.Library.AllSubmissions(ctx, c)
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.ListSharedSubmissionsOKHeaders{Response: submissionsOut(list)}, nil
}

// ReviewSharedSubmission decides a request to share, for an Admin.
func (h *Handler) ReviewSharedSubmission(ctx context.Context, req *oas.SharedReviewInput, p oas.ReviewSharedSubmissionParams) (oas.ReviewSharedSubmissionRes, error) {
	x, bad := libraryCall(ctx, h, "review submission", func(c caller.Caller) (domain.Submission, error) {
		approve := req.Decision == oas.SharedReviewInputDecisionApprove
		return h.Library.ReviewSubmission(ctx, c, uuid.UUID(p.SubmissionId), approve, req.IpClear, req.IpNote.Or(""), req.Message.Or(""))
	})
	if bad != nil {
		return bad, nil
	}
	return &oas.SharedSubmissionHeaders{Response: submissionOut(x)}, nil
}

func optUUIDOf(id oas.OptID) *uuid.UUID {
	v, ok := id.Get()
	if !ok {
		return nil
	}
	u := uuid.UUID(v)
	return &u
}

// ExportLibrary writes the caller's Homebrew in Grimoire's own schema.
func (h *Handler) ExportLibrary(ctx context.Context, p oas.ExportLibraryParams) (oas.ExportLibraryRes, error) {
	x, bad := libraryCall(ctx, h, "export library", func(c caller.Caller) (domain.Export, error) {
		return h.Library.Export(ctx, c, optUUIDOf(p.CollectionId), optUUIDOf(p.EntryId))
	})
	if bad != nil {
		return bad, nil
	}
	out := oas.LibraryExport{Format: oas.LibraryExportFormatGrimoireLibrary, Version: domain.ExportVersion, Entries: []oas.ExportedEntry{}, Collections: []oas.ExportedCollection{}}
	for _, e := range x.Entries {
		parts := []oas.ExportedEntryPartsItem{}
		if e.Design != nil {
			parts = append(parts, oas.ExportedEntryPartsItem{"type": jx.Raw(`"` + e.Kind + `"`), "design": jx.Raw(e.Design)})
		}
		out.Entries = append(out.Entries, oas.ExportedEntry{
			Key: e.Key, Kind: oas.LibraryKind(e.Kind), Name: e.Name, Fields: oas.ExportedEntryFields(e.Fields), Parts: parts,
		})
	}
	for _, c := range x.Collections {
		out.Collections = append(out.Collections, oas.ExportedCollection{Name: c.Name, Description: c.Description, Entries: c.Entries})
	}
	return &oas.LibraryExportHeaders{Response: out}, nil
}

// raw reads a JSON value an import carried, which the decoder already checked, as plain Go values.
func raw(v jx.Raw) any {
	var out any
	_ = json.Unmarshal(v, &out)
	return out
}

// ImportLibrary adds an export's entries and Collections to the caller's Library.
func (h *Handler) ImportLibrary(ctx context.Context, req *oas.LibraryImport) (oas.ImportLibraryRes, error) {
	in := make([]domain.Incoming, 0, len(req.Entries))
	for _, e := range req.Entries {
		x := domain.Incoming{Key: e.Key, Kind: e.Kind, Name: e.Name, Fields: map[string]any{}}
		for name, v := range e.Fields.Or(nil) {
			x.Fields[name] = raw(v)
		}
		for _, part := range e.Parts {
			p := map[string]any{}
			for k, v := range part {
				p[k] = raw(v)
			}
			x.Parts = append(x.Parts, p)
		}
		in = append(in, x)
	}
	cols := make([]domain.ExportedCollection, 0, len(req.Collections))
	for _, c := range req.Collections {
		cols = append(cols, domain.ExportedCollection{Name: c.Name, Description: c.Description, Entries: c.Entries})
	}
	r, bad := collectionsCallReport(ctx, h, in, cols)
	if bad != nil {
		return bad, nil
	}
	return r, nil
}

func collectionsCallReport(ctx context.Context, h *Handler, in []domain.Incoming, cols []domain.ExportedCollection) (*oas.ImportReportHeaders, *oas.ProblemStatusCodeWithHeaders) {
	r, me, bad := collectionsCall(ctx, h, "import library", func(c caller.Caller) (app.Report, error) {
		return h.Library.Import(ctx, c, in, cols)
	})
	if bad != nil {
		return nil, bad
	}
	out := oas.ImportReport{Entries: []oas.LibraryEntry{}, Collections: collectionsOut(r.Collections, me, false), Manual: []oas.ManualPart{}}
	for _, e := range r.Entries {
		out.Entries = append(out.Entries, libraryEntryOut(e))
	}
	for _, m := range r.Manual {
		out.Manual = append(out.Manual, oas.ManualPart{Where: m.Where, Reason: m.Reason})
	}
	return &oas.ImportReportHeaders{Response: out}, nil
}
