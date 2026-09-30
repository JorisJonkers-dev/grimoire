import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { describeGroup, notationFor, signed } from './notation'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const ROLL = '0190c7a8-0000-7000-8000-000000000005'
const member = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = (myRole = 'dm') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole, memberCount: 1, createdAt: '2026-09-30T20:00:00Z', me: member, members: [member],
})
const roller = { id: member.id, name: 'Joris' }
const roll = (extra: Record<string, unknown> = {}) => ({
  id: ROLL, purpose: 'Stealth', notation: '2d20kh1+1d4', requestedBy: 'Joris', roller, mine: true, canRoll: true, status: 'pending',
  groups: [
    { index: 0, count: 2, faces: 20, sign: 1, keep: 'highest', keepCount: 1, label: 'Advantage' },
    { index: 1, count: 1, faces: 4, sign: 1, label: 'Bless' },
  ],
  dice: [
    { no: 0, group: 0, faces: 20, kept: false },
    { no: 1, group: 0, faces: 20, kept: false },
    { no: 2, group: 1, faces: 4, kept: false },
  ],
  modifiers: [{ label: 'Dexterity', value: 3 }],
  createdAt: '2026-09-30T20:00:00Z',
  ...extra,
})
const withDice = (values: (number | undefined)[], extra = {}) =>
  roll({
    dice: roll().dice.map((d, i) => (values[i] === undefined ? d : { ...d, value: values[i], mode: i === 0 ? 'manual' : 'auto', kept: i !== 1 })),
    ...extra,
  })
const problem = (status: number) => () => jsonResponse({ type: 'about:blank', title: 'x', status }, status)
const base = `/api/v1/campaigns/${ID}`

function reducedMotion(on: boolean) {
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: on && q.includes('reduce'), media: q }))
}

afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('notation', () => {
  it('builds notation and labels from choices', () => {
    expect(notationFor({ count: 1, faces: 20, edge: 'advantage', bless: true, bane: true })).toEqual({
      notation: '2d20kh1+1d4-1d4',
      labels: [{ group: 0, label: 'Advantage' }, { group: 1, label: 'Bless' }, { group: 2, label: 'Bane' }],
    })
    expect(notationFor({ count: 1, faces: 20, edge: 'disadvantage', bless: false, bane: true }).notation).toBe('2d20kl1-1d4')
    expect(notationFor({ count: 8, faces: 6, edge: 'advantage', bless: false, bane: false })).toEqual({ notation: '8d6', labels: [] })
    expect(describeGroup({ index: 0, count: 4, faces: 6, sign: 1, keep: 'highest', keepCount: 3 })).toBe('4 × d6, keep highest 3')
    expect(describeGroup({ index: 1, count: 1, faces: 4, sign: -1, label: 'Bane' })).toBe('− 1 × d4 — Bane')
    expect([signed(3), signed(0), signed(-2)]).toEqual(['+3', '+0', '−2'])
  })
})

describe('dice tray', () => {
  it('requests a roll, fills dice by hand and by server, and shows the total', async () => {
    reducedMotion(true)
    const bodies: { path: string; body: unknown }[] = []
    const record = (answer: (path: string) => unknown) => async (url: URL, req: Request) => {
      bodies.push({ path: url.pathname, body: req.method === 'POST' ? await req.clone().json().catch(() => null) : null })
      return answer(url.pathname)
    }
    const { wrapper } = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls/${ROLL}/dice/0`]: record(() => withDice([12])),
      [`${base}/rolls/${ROLL}/dice/1`]: record(() => withDice([12, 17])),
      [`${base}/rolls/${ROLL}/rest`]: record(() => withDice([12, 17, 3], { status: 'resolved', total: 23, resolvedAt: '2026-09-30T20:01:00Z' })),
      [`${base}/rolls`]: async (url, req) => (req.method === 'POST' ? record(() => roll())(url, req) : [withDice([1, 2, 3], { status: 'resolved', total: 9 }), roll({ id: '0190c7a8-0000-7000-8000-000000000006' })]),
      [`${base}/log`]: () => [
        { seq: 2, kind: 'die_rolled', actor: 'Joris', origin: 'ui', seed: '42', rollId: ROLL, dieNo: 1, value: 17, createdAt: '2026-09-30T20:01:00Z' },
        { seq: 1, kind: 'roll_requested', actor: 'Joris', origin: 'ui', value: 0, createdAt: '2026-09-30T20:00:00Z' },
      ],
      [base]: () => campaign(),
    })
    await wrapper.get('[data-testid="roll-purpose"]').setValue('Stealth')
    await wrapper.get('[data-testid="roll-edge"]').setValue('advantage')
    await wrapper.get('[data-testid="roll-bless"]').setValue(true)
    await wrapper.get('[data-testid="mod-add"]').trigger('click')
    await wrapper.get('[data-testid="mod-label"]').setValue('Dexterity')
    await wrapper.get('[data-testid="mod-value"]').setValue(3)
    await wrapper.get('[data-testid="mod-add"]').trigger('click')
    expect(wrapper.get('[data-testid="roll-notation"]').text()).toBe('2d20kh1+1d4')
    await wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    expect(bodies[0]?.body).toEqual({
      purpose: 'Stealth', notation: '2d20kh1+1d4', labels: [{ group: 0, label: 'Advantage' }, { group: 1, label: 'Bless' }],
      modifiers: [{ label: 'Dexterity', value: 3 }],
    })
    const card = () => wrapper.get('[data-testid="roll-card"]')
    expect(card().text()).toContain('2 × d20, keep highest — Advantage')
    await card().get('[data-testid="manual-0"]').trigger('click')
    expect(card().findAll('[data-testid="pad-0"] button')).toHaveLength(20)
    await expectAccessible(wrapper.element as Element)
    await card().findAll('[data-testid="pad-0"] button')[11]?.trigger('click')
    await flushPromises()
    await card().get('[data-testid="auto-1"]').trigger('click')
    await flushPromises()
    await card().get('[data-testid="roll-rest"]').trigger('click')
    await flushPromises()
    expect(card().get('[data-testid="roll-total"]').text()).toContain('23')
    expect(card().text()).toContain('rolled for you')
    expect(card().get('[data-testid="die-1"] [role="img"]').attributes('aria-label')).toContain('dropped')
    expect(bodies.map((b) => b.body)).toContainEqual({ mode: 'manual', value: 12 })
    expect(bodies.map((b) => b.body)).toContainEqual({ mode: 'auto' })
    expect(wrapper.get('[data-testid="roll-history"]').text()).toContain('9')
    expect(wrapper.get('[data-testid="action-log"]').text()).toContain('seed 42')
    await wrapper.get('[data-testid="roll-history"] button').trigger('click')
    expect(card().text()).toContain('Stealth')
  })

  it('tumbles until the server answers, and reports failures', async () => {
    reducedMotion(false)
    vi.useFakeTimers()
    const { wrapper } = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls/${ROLL}/dice/0`]: problem(503),
      [`${base}/rolls/${ROLL}/rest`]: () => withDice([5, 6, 1], { status: 'resolved', total: 9 }),
      [`${base}/rolls`]: (_u, req) => (req.method === 'POST' ? roll() : []),
      [base]: () => campaign('player'),
    })
    expect(wrapper.find('[data-testid="action-log"]').exists()).toBe(false)
    await wrapper.get('[data-testid="roll-purpose"]').setValue('Stealth')
    await wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    const card = () => wrapper.get('[data-testid="roll-card"]')
    await card().get('[data-testid="auto-0"]').trigger('click')
    await vi.advanceTimersByTimeAsync(600)
    await flushPromises()
    expect(card().get('[role="alert"]').text()).toContain('could not be saved')
    await card().get('[data-testid="roll-rest"]').trigger('click')
    await vi.advanceTimersByTimeAsync(150)
    expect(card().get('[data-testid="die-2"] [role="img"]').attributes('aria-label')).toContain('rolling')
    await vi.advanceTimersByTimeAsync(600)
    await flushPromises()
    expect(card().get('[data-testid="roll-total"]').text()).toContain('9')
    wrapper.unmount()
  })

  it('shows rolls for someone else read-only and handles errors', async () => {
    reducedMotion(true)
    const { wrapper } = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls`]: (_u, req) =>
        req.method === 'POST'
          ? roll({ canRoll: false, mine: false, roller: { id: roller.id, name: 'Ireena' }, requestedBy: 'Joris', modifiers: [] })
          : [roll({ canRoll: false })],
      [base]: () => campaign('player'),
    })
    expect(wrapper.get('[data-testid="roll-history"]').text()).toContain('waiting')
    await wrapper.get('[data-testid="roll-purpose"]').setValue('Perception')
    await wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    const card = wrapper.get('[data-testid="roll-card"]')
    expect(card.text()).toContain('Waiting for Ireena')
    expect(card.text()).toContain('asked by Joris')
    expect(card.find('[data-testid="auto-0"]').exists()).toBe(false)
    document.body.innerHTML = ''
    const failing = await mountApp(`/campaigns/${ID}/dice`, { [`${base}/rolls`]: problem(422), [base]: () => campaign() })
    await failing.wrapper.get('[data-testid="roll-purpose"]').setValue('x')
    await failing.wrapper.get('[data-testid="roll-faces"]').setValue(6)
    await failing.wrapper.get('[data-testid="roll-count"]').setValue(3)
    await failing.wrapper.findAll('input[type="checkbox"]')[1]?.setValue(true)
    await failing.wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    expect(failing.wrapper.text()).toContain('could not be made')
    document.body.innerHTML = ''
    const missing = await mountApp(`/campaigns/${ID}/dice`, {})
    expect(missing.wrapper.find('[data-testid="dice-missing"]').exists()).toBe(true)
  })
})
