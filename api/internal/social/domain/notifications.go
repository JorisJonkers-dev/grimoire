package domain

import (
	"time"

	"github.com/google/uuid"
)

// Notification kinds.
const (
	KindProposal        = "proposal"
	KindJoinRequest     = "join_request"
	KindLevelUp         = "level_up"
	KindFriendRequest   = "friend_request"
	KindConversation    = "conversation"
	KindSessionReminder = "session_reminder"
	KindReleaseNote     = "release_note"
	KindSecurity        = "security"
)

// Kinds lists every Notification kind in the order the preferences show them.
func Kinds() []string {
	return []string{KindProposal, KindJoinRequest, KindLevelUp, KindFriendRequest, KindConversation, KindSessionReminder, KindReleaseNote, KindSecurity}
}

// Channels a Notification can reach an Account on.
const (
	ChannelInApp = "in_app"
	ChannelPush  = "push"
	ChannelEmail = "email"
)

// Default is whether a kind reaches a channel before the Account chooses: everything shows in app,
// everything but Release Notes reaches a device, and only security mail is sent.
func Default(kind, channel string) bool {
	switch channel {
	case ChannelInApp:
		return true
	case ChannelPush:
		return kind != KindReleaseNote
	default:
		return kind == KindSecurity
	}
}

// Notice is a Notification to send: what happened and the one thing to do about it. A Dedupe key
// replaces the Account's unread Notice with the same key.
type Notice struct {
	Kind        string
	Title       string
	Body        string
	ActionLabel string
	ActionPath  string
	Dedupe      string
	// Mail picks the email it becomes, when the kind alone does not say; empty takes the kind's.
	Mail string
}

// Recipient is where an Account's Notifications go beyond the bell.
type Recipient struct {
	Subject  string
	Email    string
	Nickname string
}

// Queued is a Notice waiting for the next Digest.
type Queued struct {
	Notice
	At time.Time
}

// Notification is a Notice as an Account's bell shows it.
type Notification struct {
	Notice
	ID     uuid.UUID
	At     time.Time
	ReadAt *time.Time
}

// Preference is whether one kind reaches each channel.
type Preference struct {
	Kind  string
	InApp bool
	Push  bool
	Email bool
}

// ReleaseNote is what one full release brought, as an Admin writes it; it goes live at PublishAt and
// is announced once.
type ReleaseNote struct {
	ID          uuid.UUID
	Version     string
	Title       string
	Body        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishAt   *time.Time
	AnnouncedAt *time.Time
}
