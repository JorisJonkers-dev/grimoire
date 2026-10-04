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
      '/api/v1/library/collections': () => [],
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
      '/api/v1/library/collections': () => [],
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
    const detail = { entry: hag, revisions: [{ no: 2, name: 'Bog Hag', fields: hag.fields, createdAt: at, origin: 'ui' }, { no: 1, name: 'Hag', fields: [], createdAt: at, origin: 'ui' }], uses: [{ campaignId: CAMP, campaign: 'Morvain', pinnedRevision: 1 }] }
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
    expect(refused.wrapper.get('[data-testid="entry-uses"]').text()).toContain('No Campaign of yours links it yet.')
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
    direct: true, via: [],
  }

  it('links, overrides, pins and unlinks entries for the Campaign', async () => {
    const sent: Sent[] = []
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/library`, {
      [`/api/v1/campaigns/${CAMP}/library`]: async (u, req) => {
        await record(sent, u, req)
        return req.method === 'GET' ? [linked] : req.method === 'DELETE' && u.pathname.endsWith(HAG) ? new Response(null, { status: 204 }) : linked
      },
      [`/api/v1/campaigns/${CAMP}/collections`]: () => [],
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

describe('collections', () => {
  const FEY = '0190c7a8-0000-7000-8000-0000000000f1'
  const THEIRS = '0190c7a8-0000-7000-8000-0000000000f2'

  it('groups entries into a Collection from the Library page', async () => {
    const sent: Sent[] = []
    const { wrapper } = await mountApp('/library', {
      '/api/v1/library/collections': async (u, req) => {
        await record(sent, u, req)
        if (req.method === 'POST') return { id: THEIRS, name: 'Undead', description: '', entryIds: [], mine: true }
        if (req.method === 'PUT') return jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'a Collection holds only entries from your own Library' }, 422)
        return [{ id: FEY, name: 'Feywild', description: 'Fey', entryIds: [HAG], mine: true }]
      },
      '/api/v1/library': () => [hag, odo],
    })
    expect(wrapper.get('[data-testid="collection-Feywild"]').text()).toContain('Feywild · 1 entry')
    await wrapper.get('[data-testid="collection-edit-Feywild"]').trigger('click')
    expect((wrapper.get('[data-testid="collect-Bog Hag"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-testid="collect-Bog Hag"]').trigger('change')
    await wrapper.get('[data-testid="collect-Odo"]').trigger('change')
    await wrapper.get('[data-testid="collection-description"]').setValue('Fey magic')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="collection-form"]').trigger('submit')
    await flushPromises()
    expect(sent.filter((x) => x.method !== 'GET').at(-1)).toEqual({
      method: 'PUT', path: `/api/v1/library/collections/${FEY}`, body: { name: 'Feywild', description: 'Fey magic', entryIds: [ODO] },
    })
    expect(wrapper.get('[data-testid="collections-problem"]').text()).toBe('a Collection holds only entries from your own Library')
    await wrapper.get('[data-testid="collection-cancel"]').trigger('click')
    await wrapper.get('[data-testid="collection-new-name"]').setValue('Undead')
    await wrapper.get('[data-testid="collection-create"]').trigger('submit')
    await flushPromises()
    expect(sent.filter((x) => x.method === 'POST').at(-1)).toEqual({ method: 'POST', path: '/api/v1/library/collections', body: { name: 'Undead' } })
  })

  it('switches Collections in a Campaign and marks what they bring in', async () => {
    const sent: Sent[] = []
    const brought: LinkedEntry = { entry: odo, baseName: 'Odo', base: [], override: [], fields: [], direct: false, via: ['Feywild'] }
    const both: LinkedEntry = { ...brought, entry: hag, baseName: 'Bog Hag', direct: true }
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/library`, {
      [`/api/v1/campaigns/${CAMP}/collections`]: async (u, req) => {
        await record(sent, u, req)
        return [{ id: FEY, name: 'Feywild', description: '', entryIds: [ODO, HAG], mine: true, switchedOn: true }, { id: THEIRS, name: 'Borrowed', description: '', entryIds: [], mine: false, switchedOn: false }]
      },
      [`/api/v1/campaigns/${CAMP}/library`]: () => [both, brought],
      '/api/v1/library': () => [hag, odo],
    })
    expect(wrapper.get('[data-testid="switch-Borrowed"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="linked-Odo"] [data-testid="via"]').text()).toBe('From Feywild')
    expect(wrapper.get('[data-testid="linked-Bog Hag"] [data-testid="via"]').text()).toBe('From Feywild and linked directly')
    expect(wrapper.find('[data-testid="linked-Odo"] [data-testid="unlink"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="link-choice"]').findAll('option').map((o) => o.text())).toEqual(['Odo (NPC)'])
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="switch-Feywild"]').setValue(false)
    await flushPromises()
    expect(sent.filter((x) => x.method === 'PUT')).toEqual([{ method: 'PUT', path: `/api/v1/campaigns/${CAMP}/collections/${FEY}`, body: { on: false } }])
  })
})

describe('shared library', () => {
  const COPY = '0190c7a8-0000-7000-8000-0000000000c1'
  const REQ = '0190c7a8-0000-7000-8000-0000000000c2'
  const copy: LibraryEntry = { ...hag, id: COPY, shared: true, revision: 1 }
  const request = {
    id: REQ, entryId: HAG, revision: 2, kind: 'creature' as const, name: 'Bog Hag', fields: hag.fields, note: 'All mine', status: 'pending' as const,
    ipNote: '', message: '', createdAt: at,
  }

  it('lists shared entries by kind', async () => {
    const { wrapper } = await mountApp('/shared-library', { '/api/v1/shared-library': () => [copy] })
    expect(wrapper.get('[data-testid="shared-list"]').text()).toContain('Bog Hag')
    await wrapper.findAll('[role="tab"]').find((t) => t.text() === 'NPC')?.trigger('click')
    expect(wrapper.get('[data-testid="shared-empty"]').text()).toBe('Nothing shared here yet.')
    await expectAccessible(wrapper.element as Element)
    const broken = await mountApp('/shared-library', { '/api/v1/shared-library': () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503) })
    expect(broken.wrapper.get('[data-testid="shared-error"]').text()).toContain('could not be opened')
  })

  it('shows a shared copy read-only, ready to link', async () => {
    const { wrapper } = await mountApp(`/library/${COPY}`, {
      '/api/v1/campaigns': () => ({ items: [{ id: OTHER, name: 'Second', ruleset: 'srd-2024', myRole: 'dm', memberCount: 1, createdAt: at }] }),
      [`/api/v1/library/${COPY}`]: () => ({ entry: copy, revisions: [{ no: 1, name: 'Bog Hag', fields: hag.fields, createdAt: at, origin: 'ui' }], uses: [] }),
    })
    expect(wrapper.get('[data-testid="entry-shared"]').text()).toContain('read-only copy')
    expect(wrapper.get('[data-testid="entry-fields"]').text()).toContain('HP')
    expect(wrapper.find('[data-testid="entry-edit"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="entry-share"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="entry-link"]').exists()).toBe(true)
  })

  it('asks the Admins to share one of the caller\'s entries', async () => {
    const sent: Sent[] = []
    const { wrapper } = await mountApp(`/library/${HAG}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      '/api/v1/shared-library/submissions': async (u, req) => {
        await record(sent, u, req)
        if (req.method === 'POST') return jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'this entry already waits for an Admin' }, 422)
        return [{ ...request, status: 'declined', message: 'Rewrite it' }, { ...request, id: OTHER, entryId: ODO }]
      },
      [`/api/v1/library/${HAG}`]: () => ({ entry: hag, revisions: [], uses: [] }),
    })
    expect(wrapper.findAll('[data-testid="share-request"]').map((li) => li.text())).toEqual(['Revision 2 · Declined · Rewrite it'])
    await wrapper.get('[data-testid="share-note"]').setValue('All mine')
    await wrapper.get('[data-testid="share"]').trigger('click')
    await flushPromises()
    expect(sent.filter((x) => x.method === 'POST')).toEqual([{ method: 'POST', path: '/api/v1/shared-library/submissions', body: { entryId: HAG, note: 'All mine' } }])
    expect(wrapper.get('[data-testid="entry-share"] [role="alert"]').text()).toBe('this entry already waits for an Admin')
  })

  it('lets an Admin share an entry only with the IP check, or decline it', async () => {
    const sent: Sent[] = []
    const { wrapper } = await mountApp('/admin/shared-library', {
      '/api/v1/admin/shared-library': async (u, req) => {
        await record(sent, u, req)
        return [request, { ...request, id: OTHER, name: 'Odo', status: 'declined', ipClear: false, ipNote: 'Quotes a book' }]
      },
    })
    const card = wrapper.get('[data-testid="request-Bog Hag"]')
    expect(card.get('[data-testid="share-approve"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="request-Odo"] [data-testid="request-outcome"]').text()).toBe('IP check not passed: Quotes a book')
    await expectAccessible(wrapper.element as Element)
    await card.get('[data-testid="ip-clear"]').setValue(true)
    await card.get('[data-testid="ip-note"]').setValue('Own words')
    await card.get('[data-testid="review-message"]').setValue('Thanks')
    await card.get('[data-testid="share-approve"]').trigger('click')
    await flushPromises()
    await card.get('[data-testid="share-decline"]').trigger('click')
    await flushPromises()
    expect(sent.filter((x) => x.method === 'POST')).toEqual([
      { method: 'POST', path: `/api/v1/admin/shared-library/${REQ}/review`, body: { decision: 'approve', ipClear: true, ipNote: 'Own words', message: 'Thanks' } },
      { method: 'POST', path: `/api/v1/admin/shared-library/${REQ}/review`, body: { decision: 'decline', ipClear: true, ipNote: 'Own words', message: 'Thanks' } },
    ])
    const refused = await mountApp('/admin/shared-library', { '/api/v1/admin/shared-library': () => jsonResponse({ type: 'about:blank', title: 'Forbidden', status: 403 }, 403) })
    expect(refused.wrapper.get('[data-testid="shared-review-forbidden"]').text()).toContain('Only an Admin')
  })
})

describe('import and export', () => {
  const FEY = '0190c7a8-0000-7000-8000-0000000000f1'
  const doc = { format: 'grimoire-library', version: 1, entries: [{ key: HAG, kind: 'creature', name: 'Bog Hag', fields: { HP: '52' }, parts: [] }], collections: [] }
  const pick = async (input: { element: Element; trigger: (e: string) => Promise<void> }, text: string) => {
    Object.defineProperty(input.element, 'files', { value: [new File([text], 'lib.json', { type: 'application/json' })], configurable: true })
    await input.trigger('change')
    await flushPromises()
  }

  it('exports everything or one Collection and imports a file with a report', async () => {
    const asked: string[] = []
    const made = vi.fn(() => 'blob:export')
    const freed = vi.fn()
    Object.assign(URL, { createObjectURL: made, revokeObjectURL: freed })
    const clicked = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    let body: unknown
    const { wrapper } = await mountApp('/library', {
      '/api/v1/library/export': (u) => {
        asked.push(u.search)
        return doc
      },
      '/api/v1/library/import': async (_u, req) => {
        body = await req.clone().json()
        return { entries: [hag], collections: [], manual: [{ where: 'entries[1] "Cart"', reason: 'the Library keeps no vehicle entries' }] }
      },
      '/api/v1/library/collections': () => [{ id: FEY, name: 'Feywild', description: '', entryIds: [], mine: true }],
      '/api/v1/library': () => [hag],
    })
    await wrapper.get('[data-testid="export-all"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="collection-export-Feywild"]').trigger('click')
    await flushPromises()
    expect(asked).toEqual(['', `?collectionId=${FEY}`])
    expect(made).toHaveBeenCalledTimes(2)
    expect(clicked).toHaveBeenCalledTimes(2)
    expect(freed).toHaveBeenCalledWith('blob:export')
    await pick(wrapper.get('[data-testid="import-file"]'), JSON.stringify(doc))
    expect(body).toEqual(doc)
    expect(wrapper.get('[data-testid="import-report"]').text()).toContain('Imported 1 entries and 0 Collections.')
    expect(wrapper.get('[data-testid="manual"]').text()).toBe('entries[1] "Cart": the Library keeps no vehicle entries')
    await expectAccessible(wrapper.element as Element)
    clicked.mockRestore()
  })

  it('says when an export fails or a file is not an export', async () => {
    const { wrapper } = await mountApp('/library', {
      '/api/v1/library/export': () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503),
      '/api/v1/library/import': () => jsonResponse({ type: 'about:blank', title: 'Bad request', status: 400 }, 400),
      '/api/v1/library/collections': () => [],
      '/api/v1/library': () => [],
    })
    await wrapper.get('[data-testid="export-all"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="transfer-problem"]').text()).toBe('The export could not be made.')
    const input = wrapper.get('[data-testid="import-file"]')
    await pick(input, 'not json')
    expect(wrapper.get('[data-testid="transfer-problem"]').text()).toBe('That file is not JSON.')
    await pick(input, '{"format":"other"}')
    expect(wrapper.get('[data-testid="transfer-problem"]').text()).toBe('That file is not a Grimoire Library export.')
    Object.defineProperty(input.element, 'files', { value: [], configurable: true })
    await input.trigger('change')
    expect(wrapper.find('[data-testid="import-report"]').exists()).toBe(false)
  })
})
