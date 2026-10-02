import type { AccountEvent } from '@/infrastructure/api/types.gen'

/** What each line of an Account's history says. */
export const eventLabels: Record<AccountEvent['action'], string> = {
  created: 'Account created',
  password_set: 'Password set',
  two_step_on: 'Two-step turned on',
  two_step_off: 'Two-step turned off',
  two_step_reset: 'Two-step reset by an Admin',
  linked: 'External login linked',
  unlinked: 'External login unlinked',
  sign_in_link_sent: 'Sign-in link emailed',
  admin_granted: 'Made an Admin',
  admin_revoked: 'Admin role removed',
  disabled: 'Disabled',
  enabled: 'Enabled again',
}

export const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
