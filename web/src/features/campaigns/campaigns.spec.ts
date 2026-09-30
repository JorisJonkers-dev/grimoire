import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, type Route } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const token = 'a'.repeat(43)
const summary = (id: string, name: string, extra = {}) => ({
  id, name, ruleset: 'srd-2024', myRole: 'dm', memberCount: 2, createdAt: '2026-09-30T20:00:00Z', ...extra,
})
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Ireena', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const home = (myRole: 'dm' | 'player') => ({
  ...summary(ID, 'Strahd', { myRole }),
  me: myRole === 'dm' ? dm : { ...player, isMe: true },
  members: myRole === 'dm' ? [dm, player] : [{ ...dm, isMe: false }, { ...player, isMe: true }],
})
const problem = (status: number) => () => jsonResponse({ type: 'about:blank', title: 'x', status }, status)
const invite = { id: '0190c7a8-0000-7000-8000-000000000006', createdAt: '2026-09-30T20:00:00Z', expiresAt: '2026-10-07T20:00:00Z', createdBy: 'Joris' }

function recorder() {
  const seen: { method: string; path: string; body: unknown }[] = []
  const record =
    (answer: (req: Request) => unknown): Route =>
    async (url, req) => {
      const text = req.method === 'GET' ? '' : await req.clone().text()
      seen.push({ method: req.method, path: url.pathname, body: text ? JSON.parse(text) : undefined })
      return answer(req)
    }
  return { seen, record }
}

afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('campaign list', () => {
  it('lists campaigns with the caller role and loads more', async () => {
    const { wrapper } = await mountApp('/campaigns', {
      '/api/v1/campaigns': (url) =>
        url.searchParams.get('cursor')
          ? { items: [summary('0190c7a8-0000-7000-8000-000000000003', 'Tomb', { myRole: 'player', memberCount: 1, ruleset: 'srd-2014' })] }
          : { items: [summary('0190c7a8-0000-7000-8000-000000000002', 'Strahd')], nextCursor: 'next' },
    })
    expect(wrapper.get('[data-testid="campaign-list"]').text()).toContain('Strahd')
    expect(wrapper.text()).toContain('2 members · 2024 rules')
    await wrapper.get('button.g-button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="campaign-list"]').text()).toContain('1 member · 2014 rules')
    expect(wrapper.get('[data-testid="campaign-list"]').text()).toContain('Player')
    await expectAccessible(wrapper.element as Element)
  })

  it('shows an empty state and an error state', async () => {
    const empty = await mountApp('/campaigns', { '/api/v1/campaigns': () => ({ items: [] }) })
    expect(empty.wrapper.find('[data-testid="campaign-empty"]').exists()).toBe(true)
    document.body.innerHTML = ''
    const broken = await mountApp('/campaigns', { '/api/v1/campaigns': problem(503) })
    expect(broken.wrapper.get('[role="alert"]').text()).toContain('could not be loaded')
  })

  it('creates a campaign and opens it', async () => {
    const { seen, record } = recorder()
    const { wrapper, router } = await mountApp('/campaigns', {
      [`/api/v1/campaigns/${ID}/invites`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => home('dm'),
      '/api/v1/campaigns': record((req) => (req.method === 'POST' ? home('dm') : { items: [] })),
    })
    const submit = wrapper.get('[data-testid="campaign-create"] button[type="submit"]')
    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="campaign-name"]').setValue(' Strahd ')
    await wrapper.get('[data-testid="campaign-display-name"]').setValue('Joris')
    await wrapper.get('[data-testid="campaign-create"] select').setValue('srd-2014')
    await wrapper.get('[data-testid="campaign-create"]').trigger('submit')
    await flushPromises()
    expect(seen.find((s) => s.method === 'POST')?.body).toEqual({ name: 'Strahd', displayName: 'Joris', ruleset: 'srd-2014' })
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('campaign') }, { timeout: 5000 })
  })

  it('reports a failed create', async () => {
    const { wrapper } = await mountApp('/campaigns', {
      '/api/v1/campaigns': (_url, req) => (req.method === 'POST' ? problem(422)() : { items: [] }),
    })
    await wrapper.get('[data-testid="campaign-name"]').setValue('Strahd')
    await wrapper.get('[data-testid="campaign-display-name"]').setValue('Joris')
    await wrapper.get('[data-testid="campaign-create"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="campaign-create"] [role="alert"]').text()).toContain('could not be created')
  })
})

describe('campaign home', () => {
  it('lets a DM invite, promote, remove and revoke', async () => {
    const writeText = vi.fn(() => Promise.resolve())
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const { seen, record } = recorder()
    const { wrapper } = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/invites`]: record((req) =>
        req.method === 'POST' ? { ...invite, token } : req.method === 'DELETE' ? new Response(null, { status: 204 }) : [invite],
      ),
      [`/api/v1/campaigns/${ID}/sessions`]: (_u, req) =>
        req.method === 'POST'
          ? { id: '0190c7a8-0000-7000-8000-00000000000b', number: 2, status: 'live', seq: 0, gridRadius: 10, startedAt: '2026-09-30T20:00:00Z' }
          : [
              { id: '0190c7a8-0000-7000-8000-00000000000b', number: 1, status: 'live', seq: 0, gridRadius: 10, startedAt: '2026-09-30T20:00:00Z' },
              { id: '0190c7a8-0000-7000-8000-00000000000c', number: 0 + 1, status: 'ended', seq: 3, gridRadius: 10, startedAt: '2026-09-30T20:00:00Z', endedAt: '2026-09-30T21:00:00Z' },
            ],
      [`/api/v1/campaigns/${ID}/characters`]: () => [
        { id: '0190c7a8-0000-7000-8000-000000000009', name: 'Kara', ownerName: 'Joris', mine: true, species: 'human', class: 'fighter', level: 1, hpCurrent: 12, hpMax: 12 },
      ],
      [`/api/v1/campaigns/${ID}/members`]: record((req) =>
        req.method === 'DELETE' ? new Response(null, { status: 204 }) : { ...player, role: 'dm' },
      ),
      [`/api/v1/campaigns/${ID}`]: () => home('dm'),
    })
    expect(wrapper.get('h1').text()).toBe('Strahd')
    expect(wrapper.get('[data-testid="party"]').text()).toContain('Kara (yours)')
    expect(wrapper.find('[data-testid="npcs-link"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="maps-link"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="sessions"]').text()).toContain('Session 1 is live')
    expect(wrapper.get('[data-testid="member-list"]').text()).toContain('Joris (you)')
    expect(wrapper.get('[data-testid="invite-list"]').text()).toContain('By Joris')
    await wrapper.get('[data-testid="create-invite"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('[data-testid="invite-link"]').element as HTMLInputElement).value).toBe(`${window.location.origin}/join#${token}`)
    await wrapper.findAll('.link button').at(0)?.trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalled()
    expect(wrapper.get('.link').text()).toContain('Copied')
    const buttons = () => wrapper.findAll('[data-testid="member-list"] button')
    await buttons().find((b) => b.text() === 'Make co-DM')?.trigger('click')
    await buttons().find((b) => b.text() === 'Make Player')?.trigger('click')
    await buttons().find((b) => b.text() === 'Remove')?.trigger('click')
    await wrapper.get('[data-testid="invite-list"] button').trigger('click')
    await flushPromises()
    const writes = seen.map((s) => `${s.method} ${s.path}`)
    expect(writes).toContain(`PATCH /api/v1/campaigns/${ID}/members/0190c7a8-0000-7000-8000-000000000005`)
    expect(writes).toContain(`PATCH /api/v1/campaigns/${ID}/members/0190c7a8-0000-7000-8000-000000000004`)
    expect(writes).toContain(`DELETE /api/v1/campaigns/${ID}/members/0190c7a8-0000-7000-8000-000000000005`)
    expect(writes).toContain(`DELETE /api/v1/campaigns/${ID}/invites/0190c7a8-0000-7000-8000-000000000006`)
    expect(seen.find((s) => s.method === 'PATCH')?.body).toEqual({ role: 'dm' })
    await expectAccessible(wrapper.element as Element)
  })

  it('explains refused changes', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/invites`]: (_url, req) => (req.method === 'GET' ? [invite] : problem(503)()),
      [`/api/v1/campaigns/${ID}/members`]: problem(409),
      [`/api/v1/campaigns/${ID}`]: () => home('dm'),
    })
    const alert = () => wrapper.get('[data-testid="campaign-error"]').text()
    await wrapper.findAll('[data-testid="member-list"] button').find((b) => b.text() === 'Make Player')?.trigger('click')
    await flushPromises()
    expect(alert()).toContain('at least one DM')
    await wrapper.findAll('[data-testid="member-list"] button').find((b) => b.text() === 'Remove')?.trigger('click')
    await flushPromises()
    expect(alert()).toContain('could not be removed')
    await wrapper.get('[data-testid="create-invite"]').trigger('click')
    await flushPromises()
    expect(alert()).toContain('could not be created')
    await wrapper.get('[data-testid="invite-list"] button').trigger('click')
    await flushPromises()
    expect(alert()).toContain('could not be revoked')
  })

  it('lets a Player see the table and leave', async () => {
    const { wrapper, router } = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/members`]: () => new Response(null, { status: 204 }),
      [`/api/v1/campaigns/${ID}`]: () => home('player'),
      '/api/v1/campaigns': () => ({ items: [] }),
    })
    expect(wrapper.find('[data-testid="invites"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="member-list"] button').exists()).toBe(false)
    expect(wrapper.text()).toContain('2024 rules · you are a Player')
    await wrapper.get('[data-testid="leave"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('campaigns')
  })

  it('hides campaigns the caller is not in', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}`, { [`/api/v1/campaigns/${ID}`]: problem(404) })
    expect(wrapper.find('[data-testid="campaign-missing"]').exists()).toBe(true)
  })
})

describe('join', () => {
  it('previews the invite and joins as a Player', async () => {
    const { seen, record } = recorder()
    const { wrapper, router } = await mountApp(`/join#${token}`, {
      '/api/v1/invites/preview': record(() => ({ campaignName: 'Strahd', invitedBy: 'Joris' })),
      '/api/v1/invites/accept': record(() => ({ id: ID })),
      [`/api/v1/campaigns/${ID}`]: () => home('player'),
    })
    expect(wrapper.get('[data-testid="join-form"]').text()).toContain('Joris invites you to Strahd')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="join-display-name"]').setValue(' Ireena ')
    await wrapper.get('[data-testid="join-form"]').trigger('submit')
    await flushPromises()
    expect(seen.map((s) => s.body)).toEqual([{ token }, { token, displayName: 'Ireena' }])
    expect(router.currentRoute.value.name).toBe('campaign')
  })

  it('rejects malformed, expired and failing invites', async () => {
    const malformed = await mountApp('/join#short', {})
    expect(malformed.wrapper.find('[data-testid="invite-invalid"]').exists()).toBe(true)
    expect(malformed.calls).toHaveLength(0)
    document.body.innerHTML = ''
    const expired = await mountApp(`/join#${token}`, { '/api/v1/invites/preview': problem(404) })
    expect(expired.wrapper.find('[data-testid="invite-invalid"]').exists()).toBe(true)
    document.body.innerHTML = ''
    const failing = await mountApp(`/join#${token}`, {
      '/api/v1/invites/preview': () => ({ campaignName: 'Strahd', invitedBy: 'Joris' }),
      '/api/v1/invites/accept': problem(404),
    })
    await failing.wrapper.get('[data-testid="join-display-name"]').setValue('Ireena')
    await failing.wrapper.get('[data-testid="join-form"]').trigger('submit')
    await flushPromises()
    expect(failing.wrapper.get('[data-testid="join-form"] [role="alert"]').text()).toContain('could not join')
  })
})

describe('table settings', () => {
  it('lets the DM choose how long reactions wait', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/sessions`]: () => [],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}/invites`]: () => [],
      [`/api/v1/campaigns/${ID}`]: async (_u, req) => {
        if (req.method === 'PATCH') {
          sent.push(await req.json())
          return { ...summary(ID, 'Strahd', { myRole: 'dm' }), reactionTimeoutS: 5 }
        }
        return { ...home('dm'), reactionTimeoutS: 20 }
      },
    })
    const input = wrapper.get('[data-testid="reaction-timeout"]')
    expect((input.element as HTMLInputElement).value).toBe('20')
    await input.setValue(5)
    await wrapper.get('[data-testid="settings"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ reactionTimeoutS: 5 }])
    expect(wrapper.get('[data-testid="settings-saved"]').text()).toBe('Saved.')
  })

  it('falls back to ten seconds and hides settings from players', async () => {
    const dmView = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/sessions`]: () => [],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}/invites`]: () => [],
      [`/api/v1/campaigns/${ID}`]: (_u, req) => (req.method === 'PATCH' ? problem(422)() : home('dm')),
    })
    expect((dmView.wrapper.get('[data-testid="reaction-timeout"]').element as HTMLInputElement).value).toBe('10')
    await dmView.wrapper.get('[data-testid="settings"]').trigger('submit')
    await flushPromises()
    expect(dmView.wrapper.text()).toContain('The settings could not be saved.')
    const playerView = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/sessions`]: () => [],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => home('player'),
    })
    expect(playerView.wrapper.find('[data-testid="settings"]').exists()).toBe(false)
  })
})

describe('sessions on the campaign home', () => {
  it('starts a session and reports failures', async () => {
    vi.stubGlobal('WebSocket', class { send() {} close() {} })
    const started = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/sessions`]: (_u, req) =>
        req.method === 'POST' ? { id: '0190c7a8-0000-7000-8000-00000000000b', number: 1, status: 'live', seq: 0, gridRadius: 10, startedAt: '2026-09-30T20:00:00Z' } : [],
      [`/api/v1/campaigns/${ID}/invites`]: () => [],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => home('dm'),
    })
    expect(started.wrapper.get('[data-testid="sessions"]').text()).toContain('No session is running')
    await started.wrapper.get('[data-testid="start-session"]').trigger('click')
    await vi.waitFor(() => { expect(started.router.currentRoute.value.name).toBe('session') }, { timeout: 5000 })
    started.wrapper.unmount()
    document.body.innerHTML = ''
    const failing = await mountApp(`/campaigns/${ID}`, {
      [`/api/v1/campaigns/${ID}/sessions`]: (_u, req) => (req.method === 'POST' ? problem(503)() : []),
      [`/api/v1/campaigns/${ID}/invites`]: () => [],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => home('dm'),
    })
    await failing.wrapper.get('[data-testid="start-session"]').trigger('click')
    await flushPromises()
    expect(failing.wrapper.get('[data-testid="campaign-error"]').text()).toContain('could not be started')
    vi.unstubAllGlobals()
  })
})
