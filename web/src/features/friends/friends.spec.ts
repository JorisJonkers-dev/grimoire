import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const me = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'a@example.com',
  admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false,
}
const person = (n: string, i: number) => ({ id: `0190c7a8-0000-7000-8000-00000000000${String(i)}`, username: n, nickname: `${n.charAt(0).toUpperCase()}${n.slice(1)}` })
const req = (n: string, i: number) => ({ id: `0190c7a8-0000-7000-8000-0000000001a${String(i)}`, person: person(n, i), at: '2026-10-02T12:00:00Z' })

afterEach(() => { unmountAll() })

describe('the Friends page', () => {
  it('sends a request, answers incoming ones, and manages Friends and blocks', async () => {
    const calls: string[] = []
    const page = {
      friends: [{ person: person('bram', 2), since: '2026-09-01T10:00:00Z' }],
      incoming: [req('cara', 3), req('dov', 4), req('eve', 5)],
      outgoing: [req('finn', 6)],
      blocked: [req('gus', 7)],
    }
    const { wrapper } = await mountApp('/friends', {
      '/api/v1/friend-requests/': async (url, req) => {
        calls.push(`${req.method} ${url.pathname.replace('/api/v1/friend-requests/', '')} ${req.method === 'POST' ? await req.text() : ''}`.trim())
        return new Response(null, { status: 204 })
      },
      '/api/v1/friend-requests': async (_u, req) => {
        const body = (await req.json()) as { username: string }
        calls.push(`ask ${body.username}`)
        return body.username === 'nobody' ? jsonResponse({ status: 404, title: 'Not found' }, 404) : new Response(null, { status: 202 })
      },
      '/api/v1/friends/': (url) => {
        calls.push(`unfriend ${url.pathname.split('/').pop() ?? ''}`)
        return new Response(null, { status: 204 })
      },
      '/api/v1/blocks/': (url) => {
        calls.push(`unblock ${url.pathname.split('/').pop() ?? ''}`)
        return new Response(null, { status: 204 })
      },
      '/api/v1/friends': () => page,
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="friends-link"]').attributes('href')).toBe('/friends')
    expect(wrapper.get('[data-testid="friend-bram"]').text()).toContain('since')
    await wrapper.get('[data-testid="friend-username"]').setValue('nobody')
    await wrapper.get('[data-testid="friend-request-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="friend-request-failed"]').text()).toContain('Nobody has that Username')
    await wrapper.get('[data-testid="friend-username"]').setValue(' Hana ')
    await wrapper.get('[data-testid="friend-request-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="friend-request-sent"]').text()).toContain('once they accept')
    await wrapper.get('[data-testid="accept-cara"]').trigger('click')
    await wrapper.get('[data-testid="decline-dov"]').trigger('click')
    await wrapper.get('[data-testid="block-eve"]').trigger('click')
    await wrapper.get('[data-testid="cancel-finn"]').trigger('click')
    await wrapper.get('[data-testid="unfriend-bram"]').trigger('click')
    await wrapper.get('[data-testid="unblock-gus"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(calls).toHaveLength(8) })
    expect(calls.slice(0, 2)).toEqual(['ask nobody', 'ask Hana'])
    // The six answers race each other, so only which went out matters.
    expect(calls.slice(2).sort()).toEqual([
      'POST 0190c7a8-0000-7000-8000-0000000001a3/accept',
      'POST 0190c7a8-0000-7000-8000-0000000001a4/decline {}',
      'POST 0190c7a8-0000-7000-8000-0000000001a5/decline {"block":true}',
      'DELETE 0190c7a8-0000-7000-8000-0000000001a6',
      'unfriend 0190c7a8-0000-7000-8000-000000000002',
      'unblock 0190c7a8-0000-7000-8000-000000000007',
    ].sort())
  })

  it('shows an empty list, and explains a missing Account', async () => {
    const empty = await mountApp('/friends', { '/api/v1/friends': () => ({ friends: [], incoming: [], outgoing: [], blocked: [] }), '/api/v1/account': () => me })
    expect(empty.wrapper.get('[data-testid="friend-none"]').text()).toContain('No Friends yet')
    unmountAll()
    const none = await mountApp('/friends', { '/api/v1/friends': () => jsonResponse({ status: 403, title: 'No Account' }, 403) })
    // The page keeps its name even when it has nothing to show.
    expect(none.wrapper.get('h1').text()).toBe('Friends')
    expect(none.wrapper.get('[data-testid="friends-no-account"]').text()).toContain('need a Grimoire Account')
  })
})
