import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { expectAccessible } from '@/test/axe'
import { fakeClock } from '@/test/mountApp'
import { GATHER_MS, HOLD_MS, THROW_MS, type ShownRoll } from './choreography'
import type { FromWorker, ToWorker } from './diceWorker'

// The GPU path's edges are faked: jsdom has neither WebGL nor workers. Playwright drives the real ones.
const gpu = vi.hoisted(() => ({
  wants: false,
  refuse: false,
  posts: [] as ToWorker[],
  stops: 0,
  flats: 0,
  answer: undefined as ((m: FromWorker) => void) | undefined,
}))
vi.mock('./gpu', () => ({
  wantsGPU: () => gpu.wants,
  goFlat: () => { gpu.flats++ },
  startWorker: (onMessage: (m: FromWorker) => void) => {
    gpu.answer = onMessage
    // Like a real worker it takes only what can be copied across; `refuse` makes it take nothing at all.
    const post = (m: ToWorker) => {
      if (gpu.refuse) throw new Error('could not be cloned')
      gpu.posts.push(m.type === 'roll' ? structuredClone(m) : m)
    }
    return { post, stop: () => { gpu.stops++ } }
  },
}))
const { default: DiceStage } = await import('./DiceStage.vue')

const athletics: ShownRoll = { roller: 'Aria', purpose: 'Athletics', dice: [{ faces: 20, value: 14, kept: true }, { faces: 20, value: 3, kept: false }], modifier: 3, total: 17 }
const crit = (value: number): ShownRoll => ({ roller: 'Aria', purpose: 'Longsword attack', dice: [{ faces: 20, value, kept: true }], modifier: 5, total: value + 5 })
const show = (roll: ShownRoll | null, n: number) => mount(DiceStage, { props: { roll, n }, attachTo: document.body })
const offscreen = { transferred: true }

beforeEach(() => {
  Object.assign(gpu, { wants: false, refuse: false, posts: [], stops: 0, flats: 0, answer: undefined })
  Object.defineProperty(HTMLCanvasElement.prototype, 'transferControlToOffscreen', { configurable: true, value: () => offscreen })
})
afterEach(() => {
  document.body.innerHTML = ''
  Reflect.deleteProperty(HTMLCanvasElement.prototype, 'transferControlToOffscreen')
})

describe('dice stage', () => {
  it('without the GPU shows the dice flat and the server\'s total at once, with criticals marked', async () => {
    fakeClock()
    const w = show(null, 0)
    expect(w.find('[data-testid="dice-canvas"]').exists()).toBe(false)
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(false)
    await w.setProps({ n: 1 })
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(false)

    await w.setProps({ roll: athletics, n: 2 })
    expect(w.get('[data-testid="dice-total"]').text()).toBe('17')
    expect(w.get('[data-testid="dice-breakdown"]').text()).toBe('14, 3 dropped + 3')
    expect(w.get('[data-testid="dice-result"]').text()).toContain('Aria · Athletics')
    expect(w.findAll('[data-testid="dice-2d"] [role="img"]').map((d) => d.attributes('aria-label'))).toEqual(['d20 showing 14, kept', 'd20 showing 3, dropped'])
    expect(w.get('[data-testid="dice-result"]').classes()).toEqual(['result'])
    expect(w.find('[data-testid="dice-critical"]').exists()).toBe(false)

    await w.setProps({ roll: crit(20), n: 3 })
    expect(w.get('[data-testid="dice-result"]').classes()).toContain('result--hit')
    expect(w.get('[data-testid="dice-critical"]').text()).toBe('Critical hit')
    expect(w.get('[data-testid="dice-total"]').text()).toBe('25')
    await w.setProps({ roll: crit(1), n: 4 })
    expect(w.get('[data-testid="dice-result"]').classes()).toContain('result--miss')
    expect(w.get('[data-testid="dice-critical"]').text()).toBe('Critical miss')
    // It stays its while, counted from the last roll, and then leaves.
    await vi.advanceTimersByTimeAsync(HOLD_MS - 1)
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(1)
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(false)
    vi.useRealTimers()
    await w.setProps({ roll: athletics, n: 5 })
    await expectAccessible(w.element as Element)
    // A roll made with a Dice Set wears it: the dice the set dresses take its colours, the others stay plain.
    const ember = { dice: { d20: { pattern: 'marble' as const, body: '#102030', numbers: '#fafafa' } } }
    await w.setProps({ roll: { ...athletics, dice: [...athletics.dice, { faces: 6, value: 2, kept: true }], look: ember }, n: 6 })
    const flat = w.findAll('[data-testid="dice-2d"] [role="img"]')
    expect(flat.map((d) => d.find('[data-testid="die-tint"]').exists())).toEqual([true, true, false])
    expect(flat[0]?.get('[data-testid="die-tint"]').attributes('fill')).toBe('#102030')
    expect([flat[0]?.get('text').attributes('fill'), flat[2]?.get('text').attributes('fill')]).toEqual(['#fafafa', '#2B2118'])
    await expectAccessible(w.element as Element)
    w.unmount()
    expect(gpu.posts).toEqual([])
  })

  it('with the GPU throws the server\'s dice and shows the total when they settle, or on time if they never say', async () => {
    gpu.wants = true
    fakeClock()
    const w = show(null, 0)
    expect(w.find('[data-testid="dice-canvas"]').exists()).toBe(true)
    expect(gpu.posts[0]).toMatchObject({ type: 'init', canvas: offscreen, width: window.innerWidth, height: window.innerHeight, dpr: 1 })
    // Before the worker is ready a roll still shows its total; there is nothing to wait for.
    await w.setProps({ roll: athletics, n: 1 })
    expect(w.get('[data-testid="dice-total"]').text()).toBe('17')
    expect(w.find('[data-testid="dice-2d"]').exists()).toBe(false)
    expect(gpu.posts).toHaveLength(1)

    gpu.answer?.({ type: 'ready' })
    // A roll that came through the page's reactive state is posted as plain data.
    await w.setProps({ roll: reactive(crit(20)), n: 2 })
    expect(gpu.posts.at(-1)).toEqual({ type: 'roll', dice: crit(20).dice, seed: 2 })
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(false)
    gpu.answer?.({ type: 'settled' })
    await flushPromises()
    expect(w.get('[data-testid="dice-total"]').text()).toBe('25')
    expect(w.get('[data-testid="dice-result"]').classes()).toContain('result--hit')
    // A late word from the worker changes nothing; when the total leaves, the dice are cleared.
    gpu.answer?.({ type: 'settled' })
    await vi.advanceTimersByTimeAsync(HOLD_MS)
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(false)
    expect(gpu.posts.at(-1)).toEqual({ type: 'clear' })

    // The roller's Dice Set goes to the worker with the dice, as plain data too.
    const ember = { dice: { d20: { pattern: 'stripes' as const, body: '#102030', numbers: '#fafafa', image: { x: 0.5, y: 0.5, scale: 1, rotation: 0 } } }, imageUrl: '/api/v1/dice-sets/x/image?v=1' }
    await w.setProps({ roll: reactive({ ...athletics, look: ember }), n: 3 })
    expect(gpu.posts.at(-1)).toEqual({ type: 'roll', dice: athletics.dice, seed: 3, look: ember })
    await vi.advanceTimersByTimeAsync(THROW_MS + GATHER_MS + HOLD_MS)

    await w.setProps({ roll: athletics, n: 4 })
    await vi.advanceTimersByTimeAsync(THROW_MS + GATHER_MS - 1)
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(false)
    await vi.advanceTimersByTimeAsync(1)
    expect(w.get('[data-testid="dice-total"]').text()).toBe('17')

    window.dispatchEvent(new Event('resize'))
    expect(gpu.posts.at(-1)).toMatchObject({ type: 'size', width: window.innerWidth })
    w.unmount()
    expect(gpu.stops).toBe(1)
    const posted = gpu.posts.length
    window.dispatchEvent(new Event('resize'))
    expect(gpu.posts).toHaveLength(posted)
  })

  it('drops to the flat dice when the device is too slow or has no WebGL, mid-throw too', async () => {
    gpu.wants = true
    fakeClock()
    const w = show(null, 0)
    gpu.answer?.({ type: 'ready' })
    await w.setProps({ roll: athletics, n: 1 })
    expect(w.find('[data-testid="dice-result"]').exists()).toBe(false)
    gpu.answer?.({ type: 'slow' })
    await flushPromises()
    expect(w.get('[data-testid="dice-total"]').text()).toBe('17')
    expect(w.find('[data-testid="dice-2d"]').exists()).toBe(true)
    expect(w.find('[data-testid="dice-canvas"]').exists()).toBe(false)
    expect([gpu.flats, gpu.stops]).toEqual([1, 1])
    await w.setProps({ roll: crit(1), n: 2 })
    expect(w.get('[data-testid="dice-critical"]').text()).toBe('Critical miss')
    expect(gpu.posts.filter((m) => m.type === 'roll')).toHaveLength(1)
    w.unmount()

    // No WebGL in the worker: flat from the start, with nothing mid-throw to finish.
    const none = show(null, 0)
    gpu.answer?.({ type: 'failed' })
    await flushPromises()
    expect(none.find('[data-testid="dice-canvas"]').exists()).toBe(false)
    expect(none.find('[data-testid="dice-result"]').exists()).toBe(false)
    none.unmount()

    // A worker that will not take the roll: flat, with the total at once.
    const deaf = show(null, 0)
    gpu.answer?.({ type: 'ready' })
    gpu.refuse = true
    await deaf.setProps({ roll: athletics, n: 1 })
    expect(deaf.get('[data-testid="dice-total"]').text()).toBe('17')
    expect(deaf.find('[data-testid="dice-2d"]').exists()).toBe(true)
    deaf.unmount()
    gpu.refuse = false

    // A canvas that cannot be handed over: the same.
    Object.defineProperty(HTMLCanvasElement.prototype, 'transferControlToOffscreen', { configurable: true, value: () => { throw new Error('no') } })
    const broken = show(athletics, 0)
    await flushPromises()
    expect(broken.find('[data-testid="dice-canvas"]').exists()).toBe(false)
    await broken.setProps({ n: 1 })
    expect(broken.get('[data-testid="dice-total"]').text()).toBe('17')
  })
})
