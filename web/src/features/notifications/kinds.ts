import type { NotificationKind } from '@/infrastructure/api/types.gen'

/** What each Notification kind is called in preferences. */
export const kindLabels: Record<NotificationKind, string> = {
  proposal: 'Proposals',
  join_request: 'Join Requests',
  level_up: 'Level-ups ready',
  friend_request: 'Friend requests',
  conversation: 'Conversations',
  session_reminder: 'Session reminders',
  release_note: 'Release Notes',
  security: 'Sign-in and security',
}
