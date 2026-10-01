import { flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { formatMonsters, parseMonsters } from './monsters'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const POOL = '0190c7a8-0000-7000-8000-000000000031'
const TABLE = '0190c7a8-0000-7000-8000-000000000032'
const TOWN = '0190c7a8-0000-7000-8000-000000000033'
const base = `/api/v1/campaigns/${ID}`
const pool = { id: POOL, name: 'Goblin band', levelMin: 1, levelMax: 4, difficulty: 'low', members: [{ monsterSlug: 'goblin', weight: 2, min: 1, max: 4 }], updatedAt: '2026-10-01T20:00:00Z' }
const table = {
  id: TABLE, name: 'Road', regionId: TOWN, chancePct: 30, visibility: 'open', updatedAt: '2026-10-01T20:00:00Z',
  entries: [
    { weight: 1, kind: 'encounter', label: 'Ambush', monsters: [{ monsterSlug: 'goblin', count: 3 }, { monsterSlug: 'ogre', count: 1 }] },
    { weight: 2, kind: 'pool', label: '', poolId: POOL, monsters: [] },
    { weight: 3, kind: 'nothing', label: 'Birdsong', monsters: [] },
    { weight: 1, kind: 'nothing', label: '', monsters: [] },
  ],
}
const revisions = [
  { no: 2, action: 'update', author: 'Joris', origin: 'ui', createdAt: '2026-10-01T21:00:00Z' },
  { no: 1, action: 'create', author: 'Joris', origin: 'ui', restoredFrom: undefined, createdAt: '2026-10-01T20:00:00Z' },
]
const checks = [
  { id: '0190c7a8-0000-7000-8000-000000000041', tableName: 'Road', trigger: 'long_rest', mode: 'normal', visibility: 'open', seed: '42', chancePct: 30, chanceRoll: 12, status: 'resolved', outcome: 'encounter', entryLabel: 'Ambush', monsters: [{ monsterSlug: 'goblin', count: 3 }], createdAt: '2026-10-01T21:00:00Z' },
  { id: '0190c7a8-0000-7000-8000-000000000042', tableName: 'Road', trigger: 'dm', mode: 'pick', visibility: 'open', seed: '7', chancePct: 30, status: 'resolved', outcome: 'nothing', entryLabel: '', monsters: [], createdAt: '2026-10-01T21:00:00Z' },
  { id: '0190c7a8-0000-7000-8000-000000000043', tableName: 'Road', trigger: 'travel_leg', mode: 'normal', visibility: 'open', seed: '8', chancePct: 30, status: 'pending', entryLabel: '', monsters: [], createdAt: '2026-10-01T21:00:00Z' },
]

describe('monster lists', () => {
  it('reads and writes creatures as the DM types them', () => {
    expect(parseMonsters(' goblin x3, Ogre ,wolf×2 ')).toEqual([{ monsterSlug: 'goblin', count: 3 }, { monsterSlug: 'ogre', count: 1 }, { monsterSlug: 'wolf', count: 2 }])
    expect(parseMonsters('')).toEqual([])
    expect(parseMonsters('goblin x')).toBeNull()
    expect(formatMonsters([{ monsterSlug: 'goblin', count: 3 }, { monsterSlug: 'ogre', count: 1 }])).toBe('goblin x3, ogre')
    expect(formatMonsters()).toBe('')
  })
})

describe('random encounters page', () => {
  it('keeps pools with their history', async () => {
    const writes: string[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/encounters`, {
      [`${base}/encounter-pools/${POOL}/revisions/1/restore`]: (_u, req) => (writes.push(`${req.method} restore`), pool),
      [`${base}/encounter-pools/${POOL}/revisions`]: () => revisions,
      [`${base}/encounter-pools/${POOL}`]: async (_u, req) => {
        writes.push(`${req.method} ${req.method === 'PUT' ? JSON.stringify(await req.json()) : ''}`)
        return req.method === 'DELETE' ? jsonResponse({ type: 'about:blank', title: 'x', status: 422, detail: 'a table still draws from this pool' }, 422) : pool
      },
      [`${base}/encounter-pools`]: async (_u, req) => {
        if (req.method === 'POST') {
          writes.push(`POST ${JSON.stringify(await req.json())}`)
          return pool
        }
        return [pool]
      },
      [`${base}/encounter-tables`]: () => [],
      [`${base}/locations`]: () => [],
      [`${base}/encounter-checks`]: () => [],
    })
    expect(wrapper.get(`[data-testid="pool-Goblin band"]`).text()).toContain('Goblin band · levels 1–4 · low · goblin 1–4')
    expect(wrapper.get('[data-testid="check-log"]').text()).toContain('No checks rolled yet.')
    await wrapper.get('[data-testid="new-pool"]').trigger('click')
    const editor = wrapper.get('[data-testid="pool-editor"]')
    expect(editor.get('[data-testid="save-pool"]').attributes('disabled')).toBeDefined()
    await editor.get('[data-testid="pool-name"]').setValue(' Wolves ')
    await editor.get('[data-testid="pool-level-min"]').setValue(2)
    await editor.get('[data-testid="pool-level-max"]').setValue(5)
    await editor.get('[data-testid="pool-difficulty"]').setValue('high')
    await editor.get('[data-testid="member-slug-0"]').setValue(' wolf ')
    await editor.get('[data-testid="add-member"]').trigger('click')
    await editor.get('[data-testid="member-slug-1"]').setValue('dire-wolf')
    await editor.get('[data-testid="member-weight-1"]').setValue(3)
    await editor.get('[data-testid="member-min-1"]').setValue(1)
    await editor.get('[data-testid="member-max-1"]').setValue(2)
    await editor.get('[aria-label="Remove creature 1"]').trigger('click')
    await expectAccessible(wrapper.element as Element)
    await editor.trigger('submit')
    await flushPromises()
    expect(writes.at(-1)).toBe('POST {"name":"Wolves","levelMin":2,"levelMax":5,"difficulty":"high","members":[{"monsterSlug":"dire-wolf","weight":3,"min":1,"max":2}]}')
    expect(wrapper.find('[data-testid="pool-editor"]').exists()).toBe(false)
    await wrapper.get('[aria-label="Edit Goblin band"]').trigger('click')
    await wrapper.get('[data-testid="pool-level-max"]').setValue(6)
    await wrapper.get('[data-testid="pool-editor"]').trigger('submit')
    await flushPromises()
    expect(writes.at(-1)).toContain('PUT {"name":"Goblin band","levelMin":1,"levelMax":6')
    await wrapper.get('[aria-label="Edit Goblin band"]').trigger('click')
    await wrapper.get('[data-testid="pool-editor"]').findAll('button').at(-1)?.trigger('click')
    expect(wrapper.find('[data-testid="pool-editor"]').exists()).toBe(false)
    await wrapper.get('[aria-label="Delete Goblin band"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="prep-error"]').text()).toBe('Deleting the pool: a table still draws from this pool')
    await wrapper.get('[aria-label="History of Goblin band"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('#2 update')
    await wrapper.get('[aria-label="Restore Goblin band to revision 1"]').trigger('click')
    await flushPromises()
    expect(writes.at(-1)).toBe('POST restore')
    expect(wrapper.find('[data-testid="prep-error"]').exists()).toBe(false)
  })

  it('keeps tables of encounters, pool draws and nothing, and the check log', async () => {
    const writes: string[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/encounters`, {
      [`${base}/encounter-tables/${TABLE}/revisions/2/restore`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503),
      [`${base}/encounter-tables/${TABLE}/revisions`]: () => revisions,
      [`${base}/encounter-tables/${TABLE}`]: async (_u, req) => {
        writes.push(`${req.method} ${req.method === 'PUT' ? JSON.stringify(await req.json()) : ''}`)
        return req.method === 'DELETE' ? new Response(null, { status: 204 }) : table
      },
      [`${base}/encounter-tables`]: async (_u, req) => {
        if (req.method === 'POST') {
          writes.push(`POST ${JSON.stringify(await req.json())}`)
          return table
        }
        return [table, { ...table, id: '0190c7a8-0000-7000-8000-000000000034', name: 'Wilds', regionId: undefined, entries: [{ weight: 1, kind: 'pool', label: '', poolId: '0190c7a8-0000-7000-8000-000000000099', monsters: [] }] }]
      },
      [`${base}/encounter-pools`]: () => [pool],
      [`${base}/locations`]: () => [{ id: TOWN, name: 'Oakford', mapName: 'Realm' }],
      [`${base}/encounter-checks`]: () => checks,
    })
    const road = wrapper.get('[data-testid="table-Road"]')
    expect(road.text()).toContain('Road · Oakford · 30% · open')
    expect(road.text()).toContain('1× Ambush (goblin x3, ogre) · 2× draw from Goblin band · 3× Birdsong · 1× nothing')
    expect(wrapper.get('[data-testid="table-Wilds"]').text()).toContain('Everywhere · 30%')
    expect(wrapper.get('[data-testid="table-Wilds"]').text()).toContain('draw from a pool')
    const log = wrapper.get('[data-testid="check-log"]').text()
    expect(log).toContain('Long rest on Road · normal · rolled 12 against 30% · Ambush: goblin x3')
    expect(log).toContain('DM on Road · pick · nothing')
    expect(log).toContain('Travel leg on Road · normal · waiting for the roll')
    expect(log).toContain('seed 42')
    await wrapper.get('[data-testid="new-table"]').trigger('click')
    const editor = wrapper.get('[data-testid="table-editor"]')
    await editor.get('[data-testid="table-name"]').setValue('Swamp')
    await editor.get('[data-testid="table-region"]').setValue(TOWN)
    await editor.get('[data-testid="table-chance"]').setValue(15)
    await editor.get('[data-testid="table-visibility"]').setValue('open')
    await editor.get('[data-testid="entry-label-0"]').setValue('Fog')
    await editor.get('[data-testid="add-entry"]').trigger('click')
    await editor.get('[data-testid="entry-monsters-1"]').setValue('goblin x')
    expect(wrapper.get('[data-testid="monsters-unreadable"]').text()).toContain('Entry 2')
    expect(editor.get('[data-testid="save-table"]').attributes('disabled')).toBeDefined()
    await editor.get('[data-testid="entry-monsters-1"]').setValue('goblin x2')
    await editor.get('[data-testid="entry-label-1"]').setValue('Bandits')
    await editor.get('[data-testid="entry-weight-1"]').setValue(4)
    await editor.get('[data-testid="add-entry"]').trigger('click')
    await editor.get('[data-testid="entry-kind-2"]').setValue('pool')
    await editor.get('[data-testid="entry-pool-2"]').setValue(POOL)
    await editor.get('[data-testid="add-entry"]').trigger('click')
    await editor.get('[aria-label="Remove entry 4"]').trigger('click')
    await expectAccessible(wrapper.element as Element)
    await editor.trigger('submit')
    await flushPromises()
    expect(JSON.parse(writes.at(-1)?.slice(5) ?? '{}')).toEqual({
      name: 'Swamp', chancePct: 15, visibility: 'open', regionId: TOWN,
      entries: [
        { weight: 1, kind: 'nothing', label: 'Fog' },
        { weight: 4, kind: 'encounter', label: 'Bandits', monsters: [{ monsterSlug: 'goblin', count: 2 }] },
        { weight: 1, kind: 'pool', label: '', poolId: POOL },
      ],
    })
    await wrapper.get('[aria-label="Edit Road"]').trigger('click')
    expect((wrapper.get('[data-testid="entry-monsters-0"]').element as HTMLInputElement).value).toBe('goblin x3, ogre')
    await wrapper.get('[data-testid="table-region"]').setValue('')
    await wrapper.get('[data-testid="table-editor"]').trigger('submit')
    await flushPromises()
    expect(writes.at(-1)).toContain('PUT {"name":"Road","chancePct":30,"visibility":"open","entries"')
    expect(writes.at(-1)).not.toContain('regionId')
    await wrapper.get('[aria-label="Edit Road"]').trigger('click')
    await wrapper.get('[data-testid="table-editor"]').findAll('button').at(-1)?.trigger('click')
    await wrapper.get('[aria-label="Delete Road"]').trigger('click')
    await flushPromises()
    expect(writes.at(-1)).toBe('DELETE ')
    await wrapper.get('[aria-label="History of Road"]').trigger('click')
    await flushPromises()
    await wrapper.get('[aria-label="Restore Road to revision 2"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="prep-error"]').text()).toBe('Restoring the table failed. Try again shortly.')
    await wrapper.get('[aria-label="History of Road"]').trigger('click')
    expect(wrapper.find('[aria-label="History of Road"]').exists()).toBe(true)
  })

  it('tells players this page is the DM\'s', async () => {
    const forbidden = () => jsonResponse({ type: 'about:blank', title: 'x', status: 403 }, 403)
    const { wrapper } = await mountApp(`/campaigns/${ID}/encounters`, {
      [`${base}/encounter-pools`]: forbidden, [`${base}/encounter-tables`]: forbidden, [`${base}/locations`]: forbidden, [`${base}/encounter-checks`]: forbidden,
    })
    expect(wrapper.get('[data-testid="encounters-refused"]').text()).toBe('Only the DM can prepare encounters.')
  })
})

describe('loot tables page', () => {
  const PURSE = '0190c7a8-0000-7000-8000-000000000051'
  const HOARD = '0190c7a8-0000-7000-8000-000000000052'
  const purse = { id: PURSE, name: 'Purse', rolls: 1, updatedAt: '2026-10-01T20:00:00Z', entries: [{ weight: 1, kind: 'currency', coin: 'gp', amount: '2d6x10' }] }
  const hoard = {
    id: HOARD, name: 'Hoard', rolls: 2, updatedAt: '2026-10-01T20:00:00Z',
    entries: [{ weight: 1, kind: 'table', tableId: PURSE }, { weight: 2, kind: 'item', itemSlug: 'rope', amount: '1d4' }, { weight: 1, kind: 'nothing' }, { weight: 1, kind: 'table', tableId: '0190c7a8-0000-7000-8000-000000000099' }],
  }
  const lootBase = `${base}/loot-tables`

  it('keeps loot tables of items, coins, other tables and nothing, with their history', async () => {
    const writes: string[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/loot`, {
      [`${lootBase}/${HOARD}/revisions/1/restore`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503),
      [`${lootBase}/${HOARD}/revisions`]: () => revisions,
      [`${lootBase}/${HOARD}`]: async (_u, req) => {
        writes.push(`${req.method} ${req.method === 'PUT' ? JSON.stringify(await req.json()) : ''}`)
        return req.method === 'DELETE' ? jsonResponse({ type: 'about:blank', title: 'x', status: 422, detail: 'another loot table still rolls on this one' }, 422) : hoard
      },
      [lootBase]: async (_u, req) => {
        if (req.method === 'POST') {
          writes.push(`POST ${JSON.stringify(await req.json())}`)
          return purse
        }
        return [hoard, purse]
      },
    })
    expect(wrapper.get('[data-testid="loot-Hoard"]').text()).toContain('Hoard · rolled 2× · 1× roll Purse · 2× 1d4 rope · 1× nothing · 1× roll a table')
    expect(wrapper.get('[data-testid="loot-Purse"]').text()).toContain('1× 2d6x10 gp')
    await wrapper.get('[data-testid="new-loot"]').trigger('click')
    await wrapper.get('[data-testid="loot-editor"]').findAll('button').at(-1)?.trigger('click')
    expect(wrapper.find('[data-testid="loot-editor"]').exists()).toBe(false)
    await wrapper.get('[data-testid="new-loot"]').trigger('click')
    const editor = wrapper.get('[data-testid="loot-editor"]')
    await editor.get('[data-testid="loot-name"]').setValue(' Chest ')
    await editor.get('[data-testid="loot-rolls"]').setValue(3)
    await editor.get('[data-testid="loot-coin-0"]').setValue('sp')
    await editor.get('[data-testid="loot-amount-0"]').setValue(' 4d6 ')
    await editor.get('[data-testid="add-loot-entry"]').trigger('click')
    await editor.get('[data-testid="loot-item-1"]').setValue(' rope ')
    await editor.get('[data-testid="loot-weight-1"]').setValue(3)
    await editor.get('[data-testid="add-loot-entry"]').trigger('click')
    await editor.get('[data-testid="loot-kind-2"]').setValue('table')
    await editor.get('[data-testid="loot-table-2"]').setValue(PURSE)
    await editor.get('[data-testid="add-loot-entry"]').trigger('click')
    await editor.get('[data-testid="loot-kind-3"]').setValue('nothing')
    await editor.get('[data-testid="add-loot-entry"]').trigger('click')
    await editor.get('[aria-label="Remove loot entry 5"]').trigger('click')
    await expectAccessible(wrapper.element as Element)
    await editor.trigger('submit')
    await flushPromises()
    expect(JSON.parse(writes.at(-1)?.slice(5) ?? '{}')).toEqual({
      name: 'Chest', rolls: 3,
      entries: [
        { weight: 1, kind: 'currency', coin: 'sp', amount: '4d6' },
        { weight: 3, kind: 'item', itemSlug: 'rope', amount: '1' },
        { weight: 1, kind: 'table', tableId: PURSE },
        { weight: 1, kind: 'nothing' },
      ],
    })
    await wrapper.get('[aria-label="Edit Hoard"]').trigger('click')
    expect(wrapper.get('[data-testid="loot-table-0"]').findAll('option').map((o) => o.text())).toEqual(['Purse'])
    await wrapper.get('[data-testid="loot-editor"]').trigger('submit')
    await flushPromises()
    expect(writes.at(-1)).toContain('PUT {"name":"Hoard","rolls":2')
    await wrapper.get('[aria-label="Edit Hoard"]').trigger('click')
    await wrapper.get('[data-testid="loot-editor"]').findAll('button').at(-1)?.trigger('click')
    await wrapper.get('[aria-label="Delete Hoard"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="loot-error"]').text()).toBe('Deleting the loot table: another loot table still rolls on this one')
    await wrapper.get('[aria-label="History of Hoard"]').trigger('click')
    await flushPromises()
    await wrapper.get('[aria-label="Restore Hoard to revision 1"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="loot-error"]').text()).toBe('Restoring the loot table failed. Try again shortly.')
  })

  it('tells players loot is the DM\'s', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/loot`, { [lootBase]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 403 }, 403) })
    expect(wrapper.get('[data-testid="loot-refused"]').text()).toBe('Only the DM can prepare loot.')
  })
})
