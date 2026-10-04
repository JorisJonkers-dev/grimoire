import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { still, vibrating } from '@/test/haptics'
import { fakeClock, mountApp } from '@/test/mountApp'
import { configureApi } from '@/infrastructure/http'
import { jsonResponse } from '@/test/mountWithQuery'
import type { RollRequest } from '@/infrastructure/api/types.gen'
import { describeGroup, notationFor, rollBreakdown, signed } from './notation'
import RollCard from './RollCard.vue'

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
  still()
  localStorage.clear()
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

describe('roll card', () => {
  const plain = (value: number, extra: object = {}) => ({
    ...roll(), notation: '1d20', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }], dice: [{ no: 0, group: 0, faces: 20, value, mode: 'manual', kept: false }],
    modifiers: [{ label: 'Longsword', value: 5 }], status: 'resolved', total: value + 5, canRoll: false, ...extra,
  })
  const show = (r: object) => mount(RollCard, { props: { roll: r as RollRequest, campaignId: ID } })

  it('keeps every face typed, sending them one after another, even while the last is still being saved', async () => {
    reducedMotion(true)
    const sent: string[] = []
    let release: () => void = () => undefined
    const held = new Promise<void>((r) => { release = r })
    configureApi({
      baseUrl: 'http://localhost',
      fetch: async (input) => {
        const req = input as Request
        const body = (await req.clone().json()) as { value: number }
        sent.push(`${new URL(req.url).pathname.split('/').at(-1) ?? ''}=${String(body.value)}`)
        if (sent.length === 1) await held
        return jsonResponse(withDice(sent.length === 1 ? [3] : [3, 4]))
      },
    })
    const w = mount(RollCard, { props: { roll: roll() as RollRequest, campaignId: ID } })
    await w.get('[data-testid="face-0"]').setValue('3')
    await w.get('[data-testid="enter-0"]').trigger('submit')
    expect((w.get('[data-testid="face-0"]').element as HTMLInputElement).value).toBe('')
    // The first is still on its way; the second is typed and entered all the same.
    await w.get('[data-testid="face-1"]').setValue('4')
    expect(w.get('[data-testid="set-1"]').attributes('disabled')).toBeUndefined()
    await w.get('[data-testid="enter-1"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual(['0=3'])
    release()
    await flushPromises()
    expect(sent).toEqual(['0=3', '1=4'])
    expect(w.emitted('updated')).toHaveLength(2)
  })

  it('carries a Standing line: its words alone when it only changes how the d20 is rolled, with its bonus when it has one', () => {
    const allied = { label: 'Allied with The Lantern Watch: Advantage', value: 0 }
    const friendly = { label: 'Friendly with The Lantern Watch', value: 2 }
    const waiting = show(roll({ modifiers: [{ label: 'Charisma', value: 3 }, allied, friendly] }))
    expect(waiting.findAll('[aria-label="Modifiers"] li').map((li) => li.text())).toEqual(['Charisma+3', 'Allied with The Lantern Watch: Advantage', 'Friendly with The Lantern Watch+2'])
    expect(rollBreakdown(plain(12, { modifiers: [allied, friendly] }) as RollRequest).slice(-2)).toEqual([{ label: allied.label, value: '' }, { label: friendly.label, value: '+2' }])
  })

  it('celebrates a natural 20 and a natural 1 on the d20 that counts', () => {
    const hit = show(plain(20))
    expect(hit.get('[data-testid="roll-natural"]').text()).toBe('Natural 20!')
    expect(hit.classes()).toContain('roll-card--hit')
    expect(hit.findAll('[data-testid="roll-breakdown"] li').map((li) => li.text())).toEqual(['d2020', 'Longsword+5'])
    const miss = show(plain(1))
    expect(miss.get('[data-testid="roll-natural"]').text()).toBe('Natural 1')
    expect(miss.classes()).toContain('roll-card--miss')
    const plainRoll = show(plain(12))
    expect(plainRoll.find('[data-testid="roll-natural"]').exists()).toBe(false)
    expect(plainRoll.classes()).toEqual(['roll-card'])
    // A d20 dropped by disadvantage does not count, and nothing is celebrated before the roll is in.
    const dropped = show({
      ...plain(20), notation: '2d20kl1', groups: [{ index: 0, count: 2, faces: 20, sign: 1, keep: 'lowest', keepCount: 1, label: 'Disadvantage' }],
      dice: [{ no: 0, group: 0, faces: 20, value: 20, kept: false }, { no: 1, group: 0, faces: 20, value: 9, kept: true }], total: 14,
    })
    expect(dropped.find('[data-testid="roll-natural"]').exists()).toBe(false)
    expect(dropped.findAll('[data-testid="roll-breakdown"] li').map((li) => li.text())).toEqual(['Disadvantage20 dropped, 9', 'Longsword+5'])
    const waiting = show(plain(20, { status: 'pending', total: undefined }))
    expect(waiting.find('[data-testid="roll-natural"]').exists()).toBe(false)
    expect(waiting.find('[data-testid="roll-breakdown"]').exists()).toBe(false)
    // Bane is taken off, and says so.
    const baned = show({
      ...plain(10), notation: '1d20-1d4', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }, { index: 1, count: 1, faces: 4, sign: -1, label: 'Bane' }],
      dice: [{ no: 0, group: 0, faces: 20, value: 10, kept: false }, { no: 1, group: 1, faces: 4, value: 3, kept: false }], modifiers: [], total: 7,
    })
    expect(baned.findAll('[data-testid="roll-breakdown"] li').map((li) => li.text())).toEqual(['d2010', 'Bane−3'])
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
    // A physical die is typed into one numeric field: no button for every face.
    const face = card().get('[data-testid="face-0"]')
    expect(face.attributes()).toMatchObject({ inputmode: 'numeric', 'aria-label': 'What your d20 shows', placeholder: '1–20' })
    expect(card().findAll('[data-testid="die-0"] button').map((b) => b.text())).toEqual(['Enter', 'Roll for me'])
    expect(card().find('[data-testid="pad-0"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
    // Only a face the die has can be entered.
    for (const wrong of ['', '0', '21', '1.5', 'x', '-3']) {
      await face.setValue(wrong)
      expect(card().get('[data-testid="set-0"]').attributes('disabled'), wrong).toBeDefined()
      await card().get('[data-testid="enter-0"]').trigger('submit')
    }
    expect(bodies.filter((b) => b.path.includes('/dice/'))).toEqual([])
    await face.setValue('12')
    expect(card().get('[data-testid="set-0"]').attributes('disabled')).toBeUndefined()
    await card().get('[data-testid="enter-0"]').trigger('submit')
    await flushPromises()
    await card().get('[data-testid="auto-1"]').trigger('click')
    await flushPromises()
    await card().get('[data-testid="roll-rest"]').trigger('click')
    await flushPromises()
    expect(card().get('[data-testid="roll-total"]').text()).toContain('23')
    expect(card().text()).toContain('rolled for you')
    // The breakdown names every part of the total: the kept d20, the dropped one, Bless and the modifier.
    expect(card().findAll('[data-testid="roll-breakdown"] li').map((li) => li.text())).toEqual(['Advantage12, 17 dropped', 'Bless+3', 'Dexterity+3'])
    expect(card().find('[data-testid="roll-natural"]').exists()).toBe(false)
    expect(card().get('[data-testid="die-1"] [role="img"]').attributes('aria-label')).toContain('dropped')
    expect(bodies.map((b) => b.body)).toContainEqual({ mode: 'manual', value: 12 })
    expect(bodies.map((b) => b.body)).toContainEqual({ mode: 'auto' })
    expect(wrapper.get('[data-testid="roll-history"]').text()).toContain('9')
    expect(wrapper.get('[data-testid="action-log"]').text()).toContain('seed 42')
    await wrapper.get('[data-testid="roll-history"] button').trigger('click')
    expect(card().text()).toContain('Stealth')
  })

  it('throws your own roll in the Dice Set you chose', async () => {
    reducedMotion(true)
    const ember = {
      id: '0190c7a8-0000-7000-8000-000000000041', name: 'Ember', hasImage: false, sharing: 'private', review: 'none', mine: true, copy: false, by: 'joris',
      updatedAt: '2026-10-03T10:00:00Z', design: { dice: { d20: { pattern: 'marble', body: '#102030', numbers: '#fafafa' } } },
    }
    const other = { ...ember, id: ember.id.replace('41', '42'), name: 'Frost', design: { dice: { d20: { pattern: 'plain', body: '#ffffff', numbers: '#000000' } } } }
    // A roll of its own: the stage throws each roll once.
    const mine = ROLL.replace(/05$/, '07')
    const { wrapper } = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls/${mine}/rest`]: () => withDice([12, 17, 3], { id: mine, status: 'resolved', total: 23, resolvedAt: '2026-09-30T20:01:00Z' }),
      [`${base}/rolls`]: (_u, req) => (req.method === 'POST' ? roll({ id: mine }) : []),
      [`${base}/log`]: () => [],
      [base]: () => campaign(),
      '/api/v1/dice-sets': () => ({ items: [other, ember], chosen: ember.id }),
    })
    await wrapper.get('[data-testid="roll-purpose"]').setValue('Stealth')
    await wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid="roll-rest"]').trigger('click')
    await flushPromises()
    // The d20s wear the chosen set; the d4, which the set leaves alone, stays plain.
    const flat = wrapper.findAll('[data-testid="dice-2d"] [role="img"]')
    expect(flat.map((d) => d.element.querySelector('[data-testid="die-tint"]')?.getAttribute('fill'))).toEqual(['#102030', '#102030', undefined])
  })

  it('lands at once when the app is set to reduce motion, and buzzes as the roll lands', async () => {
    reducedMotion(false)
    localStorage.setItem('grimoire.accessibility', JSON.stringify({ reduceMotion: true }))
    const buzzed = vibrating()
    const landed = ROLL.replace(/05$/, '09')
    const { wrapper } = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls/${landed}/dice/0`]: problem(503),
      [`${base}/rolls/${landed}/rest`]: () => withDice([5, 6, 1], { id: landed, status: 'resolved', total: 9 }),
      [`${base}/rolls`]: (_u, req) => (req.method === 'POST' ? roll({ id: landed }) : []),
      [base]: () => campaign('player'),
    })
    fakeClock()
    await wrapper.get('[data-testid="roll-purpose"]').setValue('Stealth')
    await wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    const card = () => wrapper.get('[data-testid="roll-card"]')
    // A roll that could not be saved has not landed.
    await card().get('[data-testid="auto-0"]').trigger('click')
    await flushPromises()
    expect(card().get('[role="alert"]').text()).toContain('could not be saved')
    expect(buzzed).toEqual([])
    await card().get('[data-testid="roll-rest"]').trigger('click')
    expect(card().find('[aria-label*="rolling"]').exists()).toBe(false)
    await flushPromises()
    expect(card().get('[data-testid="roll-total"]').text()).toContain('9')
    expect(buzzed).toEqual([40])
    wrapper.unmount()
  })

  it('tumbles until the server answers, and reports failures', async () => {
    reducedMotion(false)
    const { wrapper } = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls/${ROLL}/dice/0`]: problem(503),
      [`${base}/rolls/${ROLL}/rest`]: () => withDice([5, 6, 1], { status: 'resolved', total: 9 }),
      [`${base}/rolls`]: (_u, req) => (req.method === 'POST' ? roll() : []),
      [base]: () => campaign('player'),
    })
    fakeClock()
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
          ? roll({ canRoll: false, mine: false, roller: { id: roller.id, name: 'Tamsin' }, requestedBy: 'Joris', modifiers: [] })
          : [roll({ canRoll: false })],
      [base]: () => campaign('player'),
    })
    expect(wrapper.get('[data-testid="roll-history"]').text()).toContain('waiting')
    await wrapper.get('[data-testid="roll-purpose"]').setValue('Perception')
    await wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    const card = wrapper.get('[data-testid="roll-card"]')
    expect(card.text()).toContain('Waiting for Tamsin')
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

  it('offers a holder of Heroic Inspiration a reroll or the roll as it stands', async () => {
    reducedMotion(true)
    const sent: { path: string; body: unknown }[] = []
    const choosing = withDice([15, 8, 2], { choosing: true })
    const { wrapper } = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls/${ROLL}/reroll`]: async (u, req) => {
        sent.push({ path: u.pathname, body: await req.clone().json() })
        return withDice([15, 19, 2], { status: 'resolved', total: 22, rerolled: true, choosing: false })
      },
      [`${base}/rolls/${ROLL}/keep`]: (u) => {
        sent.push({ path: u.pathname, body: null })
        return withDice([15, 8, 2], { status: 'resolved', total: 20, choosing: false })
      },
      [`${base}/rolls`]: (_u, req) => (req.method === 'POST' ? choosing : []),
      [base]: () => campaign(),
    })
    await wrapper.get('[data-testid="roll-purpose"]').setValue('Stealth')
    await wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    const card = () => wrapper.get('[data-testid="roll-card"]')
    expect(card().get('[data-testid="inspiration-choice"]').text()).toContain('You have Heroic Inspiration')
    expect(card().find('[data-testid="roll-total"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
    await card().get('[data-testid="reroll-1"]').trigger('click')
    await flushPromises()
    expect(card().get('[data-testid="rerolled"]').text()).toContain('spent on a reroll')
    expect(card().get('[data-testid="roll-total"]').text()).toContain('22')
    expect(card().findAll('[data-testid="roll-breakdown"] li').map((li) => li.text())).toEqual(['Advantage15, 19 dropped', 'Bless+2', 'Dexterity+3', 'Heroic Inspirationrerolled a die'])
    expect(card().find('[data-testid="inspiration-choice"]').exists()).toBe(false)
    document.body.innerHTML = ''
    const again = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls/${ROLL}/keep`]: (u) => {
        sent.push({ path: u.pathname, body: null })
        return withDice([15, 8, 2], { status: 'resolved', total: 20, choosing: false })
      },
      [`${base}/rolls`]: (_u, req) => (req.method === 'POST' ? choosing : []),
      [base]: () => campaign(),
    })
    await again.wrapper.get('[data-testid="roll-purpose"]').setValue('Stealth')
    await again.wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    await again.wrapper.get('[data-testid="keep-roll"]').trigger('click')
    await flushPromises()
    expect(again.wrapper.get('[data-testid="roll-total"]').text()).toContain('20')
    expect(sent).toEqual([
      { path: `${base}/rolls/${ROLL}/reroll`, body: { die: 1 } },
      { path: `${base}/rolls/${ROLL}/keep`, body: null },
    ])
    document.body.innerHTML = ''
    const watching = await mountApp(`/campaigns/${ID}/dice`, {
      [`${base}/rolls`]: (_u, req) =>
        req.method === 'POST' ? { ...choosing, mine: false, canRoll: false, roller: { id: roller.id, name: 'Tamsin' } } : [],
      [base]: () => campaign('player'),
    })
    await watching.wrapper.get('[data-testid="roll-purpose"]').setValue('Stealth')
    await watching.wrapper.get('[data-testid="roll-form"]').trigger('submit')
    await flushPromises()
    const card2 = watching.wrapper.get('[data-testid="roll-card"]')
    expect(card2.text()).toContain('Tamsin may spend Heroic Inspiration')
    expect(card2.find('[data-testid="keep-roll"]').exists()).toBe(false)
    expect(card2.text()).not.toContain('Waiting for Tamsin')
  })
})
