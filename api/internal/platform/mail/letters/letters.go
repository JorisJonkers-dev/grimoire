// Package letters renders every email Grimoire sends from one dark, centred, minimal template, as
// plain text and HTML.
package letters

import (
	"bytes"
	_ "embed"
	"html/template"
	"strings"
)

// Kind is which email a Letter is.
type Kind string

// The emails Grimoire sends.
const (
	Invite           Kind = "invite"
	SignInLink       Kind = "sign_in_link"
	Confirm          Kind = "confirm"
	NewSignIn        Kind = "new_sign_in"
	TwoStepReset     Kind = "two_step_reset"
	AccountDisabled  Kind = "account_disabled"
	FriendRequest    Kind = "friend_request"
	Conversation     Kind = "conversation"
	SessionReminder  Kind = "session_reminder"
	ProposalToDM     Kind = "proposal_to_dm"
	ProposalDecision Kind = "proposal_decision"
	LevelUp          Kind = "level_up"
	ReleaseNote      Kind = "release_note"
	Digest           Kind = "digest"
)

// Kinds lists every email.
func Kinds() []Kind {
	return []Kind{Invite, SignInLink, Confirm, NewSignIn, TwoStepReset, AccountDisabled, FriendRequest, Conversation, SessionReminder, ProposalToDM, ProposalDecision, LevelUp, ReleaseNote, Digest}
}

// subjects are each email's subject line when the Letter gives none.
var subjects = map[Kind]string{ //nolint:gochecknoglobals // a fixed table
	Invite:           "You are invited to Grimoire",
	SignInLink:       "Your Grimoire sign-in link",
	Confirm:          "Confirm your email for Grimoire",
	NewSignIn:        "A new sign-in to your Grimoire Account",
	TwoStepReset:     "Your two-step sign-in was reset",
	AccountDisabled:  "Your Grimoire Account was disabled",
	FriendRequest:    "A Friend request on Grimoire",
	Conversation:     "A new message on Grimoire",
	SessionReminder:  "A Session is coming up",
	ProposalToDM:     "A Proposal for your Campaign",
	ProposalDecision: "Your Proposal has an answer",
	LevelUp:          "A Character can level up",
	ReleaseNote:      "What is new in Grimoire",
	Digest:           "What happened in Grimoire",
}

// security are the emails sent for an Account's safety, which no preference turns off.
var security = map[Kind]bool{SignInLink: true, Confirm: true, NewSignIn: true, TwoStepReset: true, AccountDisabled: true} //nolint:gochecknoglobals // a fixed table

// Security reports whether an email is sent for an Account's safety.
func Security(k Kind) bool { return security[k] }

// Action is the one button a Letter offers.
type Action struct {
	Label string
	URL   string
}

// Item is one line of a Digest.
type Item struct {
	Title string
	Body  string
	URL   string
}

// Letter is one email: who it greets, what it says, and what to do about it.
type Letter struct {
	Kind       Kind
	Subject    string
	Nickname   string
	Heading    string
	Paragraphs []string
	Action     *Action
	Items      []Item
}

//go:embed letter.html
var page string

var tmpl = template.Must(template.New("letter").Parse(page)) //nolint:gochecknoglobals // parsed once

// view is what the template reads.
type view struct {
	Letter
	Subject string
	Footer  string
}

// Render returns a Letter's subject, plain text and HTML.
func Render(l Letter) (string, string, string, error) {
	v := view{Letter: l, Subject: l.Subject, Footer: "You get this email because of your Notification preferences in Grimoire."}
	if v.Subject == "" {
		v.Subject = subjects[l.Kind]
	}
	if Security(l.Kind) {
		v.Footer = "Grimoire sends this for your Account's safety; it cannot be turned off."
	}
	var html bytes.Buffer
	if err := tmpl.Execute(&html, v); err != nil {
		return "", "", "", err
	}
	return v.Subject, text(v), html.String(), nil
}

// text is a Letter as plain text, for mail readers without HTML.
func text(v view) string {
	var b strings.Builder
	if v.Nickname != "" {
		b.WriteString("Hello " + v.Nickname + ",\n\n")
	}
	if v.Heading != "" {
		b.WriteString(v.Heading + "\n\n")
	}
	for _, p := range v.Paragraphs {
		b.WriteString(p + "\n\n")
	}
	for _, it := range v.Items {
		b.WriteString("- " + it.Title)
		if it.Body != "" {
			b.WriteString(": " + it.Body)
		}
		if it.URL != "" {
			b.WriteString("\n  " + it.URL)
		}
		b.WriteString("\n")
	}
	if len(v.Items) > 0 {
		b.WriteString("\n")
	}
	if v.Action != nil {
		b.WriteString(v.Action.Label + ": " + v.Action.URL + "\n\n")
	}
	b.WriteString("-- \n" + v.Footer + "\n")
	return b.String()
}
