package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail/letters"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// Mailer sends an email.
type Mailer interface {
	Send(ctx context.Context, m mail.Message) error
}

// Devices wakes the devices an Account opted in to push on.
type Devices interface {
	Push(subject, title, body, url string)
}

// DigestEvery is how often an Account gets at most one Digest.
const DigestEvery = time.Hour

// mailKinds are the emails each Notification kind becomes. A Join Request asks the DM, like a Proposal.
var mailKinds = map[string]letters.Kind{ //nolint:gochecknoglobals // a fixed table
	domain.KindProposal:        letters.ProposalToDM,
	domain.KindJoinRequest:     letters.ProposalToDM,
	domain.KindLevelUp:         letters.LevelUp,
	domain.KindFriendRequest:   letters.FriendRequest,
	domain.KindConversation:    letters.Conversation,
	domain.KindSessionReminder: letters.SessionReminder,
	domain.KindReleaseNote:     letters.ReleaseNote,
	domain.KindSecurity:        letters.NewSignIn,
}

func (s *Service) link(path string) string {
	return strings.TrimRight(s.BaseURL, "/") + path
}

// letter is the email one Notice becomes.
func letter(to domain.Recipient, n domain.Notice, url string) letters.Letter {
	kind := letters.Kind(n.Mail)
	if kind == "" {
		kind = mailKinds[n.Kind]
	}
	l := letters.Letter{Kind: kind, Subject: n.Title, Nickname: to.Nickname, Heading: n.Title, Paragraphs: nil, Action: &letters.Action{Label: n.ActionLabel, URL: url}, Items: nil}
	if n.Body != "" {
		l.Paragraphs = []string{n.Body}
	}
	return l
}

func (s *Service) mail(ctx context.Context, to domain.Recipient, l letters.Letter) error {
	subject, text, html, err := letters.Render(l)
	if err != nil {
		return err
	}
	return s.Mailer.Send(ctx, mail.Message{To: to.Email, Subject: subject, Text: text, HTML: html})
}

// SendDigests emails every Account whose queue waits and who had no Digest within the hour: a single
// Notice as its own email, several as one Digest. Each Account's queue empties only when its email
// leaves.
func (s *Service) SendDigests(ctx context.Context) error {
	if s.Mailer == nil {
		return nil
	}
	now := s.Now()
	due, err := s.Repo.DueDigests(ctx, now.Add(-DigestEvery))
	if err != nil {
		return err
	}
	var errs []error
	for _, account := range due {
		errs = append(errs, s.Repo.InTx(ctx, func(r Repository) error { return s.digest(ctx, r, account, now) }))
	}
	return errors.Join(errs...)
}

func (s *Service) digest(ctx context.Context, r Repository, account domain.AccountID, now time.Time) error {
	queued, err := r.TakeQueued(ctx, account)
	if err != nil {
		return err
	}
	to, err := r.Recipient(ctx, account)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && to.Email == "") || len(queued) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	l := letter(to, queued[0].Notice, s.link(queued[0].ActionPath))
	if len(queued) > 1 {
		l = letters.Letter{Kind: letters.Digest, Subject: "", Nickname: to.Nickname, Heading: "Since your last Digest", Paragraphs: nil, Action: &letters.Action{Label: "Open Grimoire", URL: s.link("/")}, Items: nil}
		for _, q := range queued {
			l.Items = append(l.Items, letters.Item{Title: q.Title, Body: q.Body, URL: s.link(q.ActionPath)})
		}
	}
	if err := s.mail(ctx, to, l); err != nil {
		return err
	}
	return r.MarkDigest(ctx, account, now)
}
