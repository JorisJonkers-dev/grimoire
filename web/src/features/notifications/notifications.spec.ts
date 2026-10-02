import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'

const me = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'a@example.com',
  admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false,
}
const friend = { id: '0190c7a8-0000-7000-8000-0000000000b1', kind: 'friend_request', title: 'bram wants to be Friends', body: '@bram', actionLabel: 'Review', actionPath: '/friends', at: '2026-10-02T12:00:00Z', read: false }
const talk = { id: '0190c7a8-0000-7000-8000-0000000000b2', kind: 'conversation', title: 'bram wrote to you', body: 'See you', actionLabel: 'Open', actionPath: '/conversations', at: '2026-10-02T11:00:00Z', read: true }
const prefs = { items: ['proposal', 'join_request', 'level_up', 'friend_request', 'conversation', 'session_reminder', 'release_note', 'security'].map((kind) => ({ kind, inApp: true, push: kind !== 'release_note', email: kind === 'security' })) }

afterEach(() => { unmountAll() })

describe('the bell', () => {
  it('counts unread Notifications, acts on one and marks them read', async () => {
    const calls: string[] = []
    let unread = 1
    const { wrapper, router } = await mountApp('/campaigns', {
      '/api/v1/notifications/read': () => {
        calls.push('all')
        unread = 0
        return new Response(null, { status: 204 })
      },
      '/api/v1/notifications/': (url) => {
        calls.push(url.pathname)
        return new Response(null, { status: 204 })
      },
      '/api/v1/notifications': () => ({ items: [friend, talk], unread }),
      '/api/v1/friends': () => ({ friends: [], incoming: [], outgoing: [], blocked: [] }),
      '/api/v1/campaigns': () => ({ items: [] }),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="bell-count"]').text()).toBe('1')
    expect(wrapper.get('[data-testid="bell"]').attributes('aria-label')).toBe('Notifications, 1 unread')
    await wrapper.get('[data-testid="bell"]').trigger('click')
    expect(wrapper.get('[data-testid="bell-panel"]').text()).toContain('bram wants to be Friends')
    await wrapper.get('[data-testid="bell-read-all"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(wrapper.find('[data-testid="bell-count"]').exists()).toBe(false) })
    await wrapper.get(`[data-testid="act-${friend.id}"]`).trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(router.currentRoute.value.fullPath).toBe('/friends') })
    expect(calls).toEqual(['all', `/api/v1/notifications/${friend.id}/read`])
    await wrapper.get('[data-testid="bell"]').trigger('click')
    await wrapper.get(`[data-testid="act-${talk.id}"]`).trigger('click')
    await flushPromises()
    expect(calls).toHaveLength(2)
  })

  it('says when nothing is new', async () => {
    const { wrapper } = await mountApp('/campaigns', {
      '/api/v1/notifications': () => ({ items: [], unread: 0 }),
      '/api/v1/campaigns': () => ({ items: [] }),
      '/api/v1/account': () => me,
    })
    await wrapper.get('[data-testid="bell"]').trigger('click')
    expect(wrapper.get('[data-testid="bell-empty"]').text()).toBe('Nothing new.')
  })
})

describe('Notification preferences', () => {
  it('chooses channels per kind, with security always in app', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp('/account', {
      '/api/v1/notification-preferences': async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.json()) as typeof prefs
          sent.push(body)
          return body
        }
        return prefs
      },
      '/api/v1/notifications': () => ({ items: [], unread: 0 }),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="pref-security-in-app"]').attributes('disabled')).toBeDefined()
    expect((wrapper.get('[data-testid="pref-release_note-push"]').element as HTMLInputElement).checked).toBe(false)
    await wrapper.get('[data-testid="pref-conversation-email"]').setValue(true)
    await wrapper.get('[data-testid="pref-friend_request-in-app"]').setValue(false)
    await wrapper.get('[data-testid="notification-preferences"]').trigger('submit')
    await flushPromises()
    const items = (sent[0] as typeof prefs).items
    expect(items.find((p) => p.kind === 'conversation')).toEqual({ kind: 'conversation', inApp: true, push: true, email: true })
    expect(items.find((p) => p.kind === 'friend_request')?.inApp).toBe(false)
    expect(wrapper.get('[data-testid="preferences-saved"]').text()).toBe('Saved.')
  })
})
