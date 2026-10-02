import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { LibraryEntry, LinkedEntry } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { cleanFields } from './fields'

const HAG = '0190c7a8-0000-7000-8000-0000000000e1'
const ODO = '0190c7a8-0000-7000-8000-0000000000e2'
const CAMP = '0190c7a8-0000-7000-8000-000000000001'
const OTHER = '0190c7a8-0000-7000-8000-000000000002'
const at = '2026-10-01T20:00:00Z'
const hag: LibraryEntry = {
  id: HAG, kind: 'creature', name: 'Bog Hag', revision: 2, createdAt: at, updatedAt: at,
  fields: [{ name: 'AC', value: '17' }, { name: 'HP', value: '52' }],
}
const odo: LibraryEntry = { id: ODO, kind: 'npc', name: 'Odo', revision: 1, createdAt: at, updatedAt: at, fields: [] }
type Sent = { method: string; path: string; body?: unknown }

async function record(sent: Sent[], u: URL, req: Request) {
  sent.push({ method: req.method, path: u.pathname, body: req.method === 'GET' || req.method === 'DELETE' ? undefined : await req.clone().json() })
}

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('library helpers', () => {
  it('keeps named rows, trimmed, the last value of a repeated name', () => {
    expect(cleanFields([{ name: ' HP ', value: '5' }, { name: '', value: 'x' }, { name: 'HP', value: '7' }, { name: 'AC', value: '' }])).toEqual([
      { name: 'HP', value: '7' },
      { name: 'AC', value: '' },
    ])
  })
})

describe('library page', () => {
  it('lists entries by kind and adds one with its fields', async () => {
    const sent: Sent[] = []
    const { wrapper, router } = await mountApp('/library', {
      [`/api/v1/library/${HAG}`]: () => ({ entry: hag, revisions: [], uses: [] }),
      '/api/v1/library': async (u, req) => {
        await record(sent, u, req)
        return req.method === 'POST' ? hag : [hag, odo]
      },
    })
    expect(wrapper.findAll('[data-testid="library-list"] li')).toHaveLength(2)
    await wrapper.findAll('[role="tab"]').find((t) => t.text() === 'NPC')?.trigger('click')
    expect(wrapper.get('[data-testid="library-list"]').text()).toContain('Odo')
    expect(wrapper.get('[data-testid="library-list"]').text()).not.toContain('Bog Hag')
    await wrapper.findAll('[role="tab"]').find((t) => t.text() === 'Shop')?.trigger('click')
    expect(wrapper.get('[data-testid="library-empty"]').text()).toBe('Nothing here yet.')
    await expectAccessible(wrapper.element as Element)

    await wrapper.get('[data-testid="library-kind"]').setValue('creature')
    await wrapper.get('[data-testid="library-name"]').setValue('Bog Hag')
    await wrapper.get('[data-testid="field-add"]').trigger('click')
    await wrapper.get('[data-testid="field-add"]').trigger('click')
    await wrapper.get('[data-testid="field-name-0"]').setValue('HP')
    await wrapper.get('[data-testid="field-value-0"]').setValue('52')
    await wrapper.get('[data-testid="field-remove-1"]').trigger('click')
    expect(wrapper.find('[data-testid="field-name-1"]').exists()).toBe(false)
    await wrapper.get('[data-testid="library-create"]').trigger('submit')
    await flushPromises()
    expect(sent.filter((x) => x.method !== 'GET').at(-1)).toEqual({ method: 'POST', path: '/api/v1/library', body: { kind: 'creature', name: 'Bog Hag', fields: [{ name: 'HP', value: '52' }] } })
    await vi.waitFor(() => {
      expect(router.currentRoute.value.name).toBe('library-entry')
    })
  })

  it('says when the Library cannot be opened, and when a save is refused', async () => {
    const { wrapper } = await mountApp('/library', {
      '/api/v1/library': (_u, req) =>
        req.method === 'POST'
          ? jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'give it a name of up to 80 characters' }, 422)
          : [],
    })
    await wrapper.get('[data-testid="library-name"]').setValue('X')
    await wrapper.get('[data-testid="library-create"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="library-create"] [role="alert"]').text()).toBe('give it a name of up to 80 characters')
    const broken = await mountApp('/library', { '/api/v1/library': () => jsonResponse({ type: 'about:blank', title: 'Unavailable', status: 503 }, 503) })
    expect(broken.wrapper.get('[data-testid="library-error"]').text()).toContain('could not be opened')
  })
})

describe('library entry page', () => {
  it('edits the base as a new Revision, shows where it is used and links it into a Campaign', async () => {
    const sent: Sent[] = []
    const detail = { entry: hag, revisions: [{ no: 2, name: 'Bog Hag', fields: hag.fields, createdAt: at }, { no: 1, name: 'Hag', fields: [], createdAt: at }], uses: [{ campaignId: CAMP, campaign: 'Morvain', pinnedRevision: 1 }] }
    const { wrapper } = await mountApp(`/library/${HAG}`, {
      [`/api/v1/campaigns/${OTHER}/library`]: async (u, req) => {
        await record(sent, u, req)
        return {}
      },
      '/api/v1/campaigns': () => ({
        items: [
          { id: CAMP, name: 'Morvain', ruleset: 'srd-2024', myRole: 'dm', memberCount: 2, createdAt: at },
          { id: OTHER, name: 'Second', ruleset: 'srd-2024', myRole: 'dm', memberCount: 1, createdAt: at },
          { id: '0190c7a8-0000-7000-8000-000000000003', name: 'Played', ruleset: 'srd-2024', myRole: 'player', memberCount: 3, createdAt: at },
        ],
      }),
      [`/api/v1/library/${HAG}`]: async (u, req) => {
        await record(sent, u, req)
        return req.method === 'PUT' ? { ...detail, entry: { ...hag, name: 'Bog Hag Matriarch', revision: 3 } } : detail
      },
    })
    expect(wrapper.get('[data-testid="entry-uses"]').text()).toContain('Morvain · pinned to Revision 1')
    expect(wrapper.get('[data-testid="entry-revisions"]').text()).toContain('Revision 1 · Hag')
    expect(wrapper.get('[data-testid="entry-link-target"]').findAll('option').map((o) => o.text())).toEqual(['Second'])
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="entry-name"]').setValue('Bog Hag Matriarch')
    await wrapper.get('[data-testid="field-value-1"]').setValue('80')
    await wrapper.get('[data-testid="field-add"]').trigger('click')
    await wrapper.get('[data-testid="entry-edit"]').trigger('submit')
    await flushPromises()
    expect(sent.filter((x) => x.method !== 'GET').at(-1)).toEqual({
      method: 'PUT', path: `/api/v1/library/${HAG}`, body: { name: 'Bog Hag Matriarch', fields: [{ name: 'AC', value: '17' }, { name: 'HP', value: '80' }] },
    })
    expect(wrapper.get('[data-testid="entry-status"]').text()).toBe('Saved as Revision 3.')
    await wrapper.get('[data-testid="entry-link-target"]').setValue(OTHER)
    await wrapper.get('[data-testid="entry-link"]').trigger('click')
    await flushPromises()
    expect(sent.filter((x) => x.method !== 'GET').at(-1)).toEqual({ method: 'POST', path: `/api/v1/campaigns/${OTHER}/library`, body: { entryId: HAG } })
  })

  it('says when the entry is not the caller\'s, and when an edit is refused', async () => {
    const { wrapper } = await mountApp(`/library/${HAG}`, { '/api/v1/campaigns': () => ({ items: [] }) })
    expect(wrapper.get('[data-testid="entry-error"]').text()).toContain('no such entry')
    const refused = await mountApp(`/library/${HAG}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/library/${HAG}`]: (_u, req) =>
        req.method === 'PUT'
          ? jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'keep it to 100 fields' }, 422)
          : { entry: hag, revisions: [], uses: [] },
    })
    expect(refused.wrapper.get('[data-testid="entry-uses"]').text()).toContain('No Campaign links it yet.')
    await refused.wrapper.get('[data-testid="entry-edit"]').trigger('submit')
    await flushPromises()
    expect(refused.wrapper.get('[data-testid="entry-edit"] [role="alert"]').text()).toBe('keep it to 100 fields')
  })
})

describe('campaign library page', () => {
  const linked: LinkedEntry = {
    entry: hag, pinnedRevision: 1, baseName: 'Hag', base: [{ name: 'AC', value: '17' }],
    override: [{ name: 'AC', value: '15' }, { name: 'Mood', value: 'wounded' }],
    fields: [{ name: 'AC', value: '15' }, { name: 'Mood', value: 'wounded' }],
  }

  it('links, overrides, pins and unlinks entries for the Campaign', async () => {
    const sent: Sent[] = []
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/library`, {
      [`/api/v1/campaigns/${CAMP}/library`]: async (u, req) => {
        await record(sent, u, req)
        return req.method === 'GET' ? [linked] : req.method === 'DELETE' && u.pathname.endsWith(HAG) ? new Response(null, { status: 204 }) : linked
      },
      [`/api/v1/campaigns/${CAMP}`]: () => ({ id: CAMP, name: 'Morvain', ruleset: 'srd-2024', myRole: 'dm', memberCount: 1, createdAt: at }),
      '/api/v1/library': () => [hag, odo],
    })
    const card = wrapper.get('[data-testid="linked-Bog Hag"]')
    expect(card.get('h2').text()).toContain('Hag')
    expect(card.get('[data-testid="value-AC"]').text()).toContain('15')
    expect(card.get('[data-testid="value-AC"] .g-tag').attributes('title')).toBe('Base: 17')
    expect(card.get('[data-testid="value-Mood"] .g-tag').attributes('title')).toBe('Only in this Campaign')
    expect(wrapper.get('[data-testid="link-choice"]').findAll('option').map((o) => o.text())).toEqual(['Odo (NPC)'])
    await expectAccessible(wrapper.element as Element)

    await wrapper.get('[data-testid="link-choice"]').setValue(ODO)
    await wrapper.get('[data-testid="link-entry"]').trigger('click')
    await card.get('[data-testid="override-edit"]').trigger('click')
    await card.get('[data-testid="field-remove-1"]').trigger('click')
    await card.get('[data-testid="override-save"]').trigger('click')
    await flushPromises()
    await card.get('[data-testid="override-edit"]').trigger('click')
    await card.get('[data-testid="override-cancel"]').trigger('click')
    await card.get('[data-testid="pin"]').setValue('2')
    await flushPromises()
    await card.get('[data-testid="pin"]').setValue('')
    await flushPromises()
    await card.get('[data-testid="unlink"]').trigger('click')
    await flushPromises()
    expect(sent.filter((s) => s.method !== 'GET')).toEqual([
      { method: 'POST', path: `/api/v1/campaigns/${CAMP}/library`, body: { entryId: ODO } },
      { method: 'PUT', path: `/api/v1/campaigns/${CAMP}/library/${HAG}/override`, body: { fields: [{ name: 'AC', value: '15' }] } },
      { method: 'PUT', path: `/api/v1/campaigns/${CAMP}/library/${HAG}/pin`, body: { revision: 2 } },
      { method: 'DELETE', path: `/api/v1/campaigns/${CAMP}/library/${HAG}/pin` },
      { method: 'DELETE', path: `/api/v1/campaigns/${CAMP}/library/${HAG}` },
    ])
  })

  it('refuses a player and reports a failed change', async () => {
    const refused = await mountApp(`/campaigns/${CAMP}/library`, {
      [`/api/v1/campaigns/${CAMP}/library`]: () => jsonResponse({ type: 'about:blank', title: 'Forbidden', status: 403 }, 403),
      '/api/v1/library': () => [],
    })
    expect(refused.wrapper.get('[data-testid="links-refused"]').text()).toContain('Only the DM')
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/library`, {
      [`/api/v1/campaigns/${CAMP}/library`]: (_u, req) =>
        req.method === 'GET' ? [] : jsonResponse({ type: 'about:blank', title: 'Not found', status: 404, detail: 'No such Library entry, Campaign or link.' }, 404),
      '/api/v1/library': () => [odo],
    })
    expect(wrapper.get('[data-testid="links-empty"]').text()).toBe('Nothing linked yet.')
    await wrapper.get('[data-testid="link-entry"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="links-problem"]').text()).toBe('No such Library entry, Campaign or link.')
  })
})
