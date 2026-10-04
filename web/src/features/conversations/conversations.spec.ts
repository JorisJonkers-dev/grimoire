import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const me = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'a@example.com',
  admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false,
}
const bram = { id: '0190c7a8-0000-7000-8000-0000000000c2', username: 'bram', nickname: 'Bram' }
const dana = { id: '0190c7a8-0000-7000-8000-0000000000c3', username: 'dana', nickname: 'Dana' }
const talk = '0190c7a8-0000-7000-8000-0000000000d1'
const camp = '0190c7a8-0000-7000-8000-0000000000e1'
const kara = '0190c7a8-0000-7000-8000-0000000000f1'
const salt = '0190c7a8-0000-7000-8000-0000000000f2'
const world = '0190c7a8-0000-7000-8000-0000000000f3'
const friends = { friends: [{ person: bram, since: '2026-09-01T10:00:00Z' }, { person: dana, since: '2026-09-01T10:00:00Z' }], incoming: [], outgoing: [], blocked: [] }

afterEach(() => { unmountAll() })

describe('Conversations', () => {
  it('lists Conversations with unread counts and starts a group with Friends', async () => {
    const sent: unknown[] = []
    const { wrapper, router } = await mountApp('/conversations', {
      '/api/v1/conversations': async (_u, req) => {
        if (req.method === 'POST') {
          sent.push(await req.json())
          return jsonResponse({ id: talk }, 201)
        }
        return { items: [{ id: talk, title: '', members: [me, bram], updatedAt: '2026-10-02T12:00:00Z', unread: 3, lastBody: 'See you Friday' }] }
      },
      '/api/v1/friends': () => friends,
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="conversations-link"]').attributes('aria-label')).toBe('Talk')
    const row = wrapper.get(`[data-testid="conversation-${talk}"]`).text()
    expect(row).toContain('Aria, Bram')
    expect(row).toContain('See you Friday')
    expect(wrapper.get('[data-testid="conversation-list"]').text()).toContain('3')
    await wrapper.get('[data-testid="pick-bram"]').setValue(true)
    await wrapper.get('[data-testid="pick-dana"]').setValue(true)
    await wrapper.get('[data-testid="conversation-title"]').setValue(' Party ')
    await wrapper.get('[data-testid="conversation-start"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ with: [bram.id, dana.id], title: 'Party' }])
    await vi.waitFor(() => { expect(router.currentRoute.value.fullPath).toBe(`/conversations/${talk}`) })
  })

  it('shows a thread with Mentions as links or shut, and sends a message with a Mention', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp(`/conversations/${talk}`, {
      [`/api/v1/conversations/${talk}/messages`]: async (_u, req) => {
        if (req.method === 'POST') {
          sent.push(await req.json())
          return jsonResponse({ id: '0190c7a8-0000-7000-8000-0000000000a9', author: me, body: 'x', at: '2026-10-02T12:00:00Z', mentions: [] }, 201)
        }
        return { items: [
          { id: '0190c7a8-0000-7000-8000-0000000000a2', author: bram, body: 'Bring @Kara to @Saltmarsh', at: '2026-10-02T11:00:00Z', mentions: [
            { kind: 'character', campaignId: camp, id: kara, open: true, label: 'Kara' },
            { kind: 'location', campaignId: camp, id: salt, open: false },
          ] },
          { id: '0190c7a8-0000-7000-8000-0000000000a1', author: me, body: 'Hello', at: '2026-10-02T10:00:00Z', mentions: [{ kind: 'location', campaignId: camp, id: world, open: true, label: 'The Reach', mapId: world }] },
        ] }
      },
      '/api/v1/mentionables': (url) => ({ items: url.searchParams.get('q') ? [{ kind: 'character', id: kara, name: 'Kara', campaignId: camp, campaignName: 'Morvain' }] : [] }),
      '/api/v1/account': () => me,
    })
    const thread = wrapper.get('[data-testid="thread"]').text()
    expect(thread.indexOf('Hello')).toBeLessThan(thread.indexOf('Bring'))
    expect(wrapper.get(`[data-testid="mention-${kara}"]`).attributes('href')).toBe(`/campaigns/${camp}/characters/${kara}`)
    expect(wrapper.get(`[data-testid="mention-${world}"]`).attributes('href')).toBe(`/campaigns/${camp}/maps/${world}`)
    expect(wrapper.get(`[data-testid="mention-${salt}"]`).text()).toBe('Something you cannot open')
    await wrapper.get('[data-testid="message-body"]').setValue('Who brings')
    await wrapper.get('[data-testid="mention-open"]').trigger('click')
    await wrapper.get('[data-testid="mention-search"]').setValue('ka')
    await flushPromises()
    await wrapper.get(`[data-testid="pick-mention-${kara}"]`).trigger('click')
    expect(wrapper.get('[data-testid="composer-mentions"]').text()).toBe('Kara')
    await wrapper.get('[data-testid="composer"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ body: 'Who brings @Kara', mentions: [{ kind: 'character', campaignId: camp, id: kara }] }])
  })

  it('says when a Conversation is not yours, and when there are none', async () => {
    const shut = await mountApp(`/conversations/${talk}`, { '/api/v1/conversations': () => jsonResponse({ status: 404, title: 'Not found' }, 404), '/api/v1/account': () => me })
    expect(shut.wrapper.get('[data-testid="conversation-error"]').text()).toContain('could not be read')
    unmountAll()
    const none = await mountApp('/conversations', {
      '/api/v1/conversations': () => ({ items: [] }),
      '/api/v1/friends': () => ({ friends: [], incoming: [], outgoing: [], blocked: [] }),
      '/api/v1/account': () => me,
    })
    expect(none.wrapper.get('[data-testid="conversation-none"]').text()).toContain('No Conversations')
    expect(none.wrapper.text()).toContain('Friends page')
  })

  it('opens a Conversation from the Friends page', async () => {
    const { wrapper, router } = await mountApp('/friends', {
      [`/api/v1/conversations/${talk}/messages`]: () => ({ items: [] }),
      '/api/v1/conversations': () => jsonResponse({ id: talk }, 201),
      '/api/v1/friends': () => friends,
      '/api/v1/account': () => me,
    })
    await wrapper.get('[data-testid="talk-bram"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(router.currentRoute.value.fullPath).toBe(`/conversations/${talk}`) })
    await flushPromises()
    expect(wrapper.get('[data-testid="thread-empty"]').text()).toContain('Say hello')
  })
})
