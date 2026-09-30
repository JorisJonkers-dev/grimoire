import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import HexGrid from './HexGrid.vue'
import { key, sandbox } from './sandbox'

afterEach(() => {
  document.body.innerHTML = ''
})

describe('HexGrid', () => {
  it('draws one polygon per cell and reports clicks and Enter', async () => {
    const w = mount(HexGrid, { props: { cells: [{ q: 0, r: 0, tone: 'start', label: 'You' }, { q: 1, r: 0 }], title: 'Map' }, attachTo: document.body })
    expect(w.findAll('polygon')).toHaveLength(2)
    expect(w.get('[data-hex="0,0"]').attributes('aria-label')).toBe('Hex 0, 0: You')
    await w.get('[data-hex="1,0"]').trigger('click')
    await w.get('[data-hex="0,0"]').trigger('keydown', { key: 'Enter' })
    expect(w.emitted('select')).toEqual([[{ q: 1, r: 0 }], [{ q: 0, r: 0 }]])
    expect(Number(w.get('svg').attributes('width'))).toBeGreaterThan(80)
    await expectAccessible(w.element as Element)
    const empty = mount(HexGrid, { props: { cells: [], title: 'Empty' } })
    expect(empty.get('svg').attributes('viewBox')).toBe('-24.0 -24.0 48.0 48.0')
  })

  it('builds the sandbox field', () => {
    const f = sandbox()
    expect(f.cells).toHaveLength(61)
    expect(f.cells.find((c) => key(c) === '0,-1')?.blocked).toBe(true)
    expect(f.cells.find((c) => key(c) === '2,0')?.difficult).toBe(true)
  })
})

describe('movement sandbox', () => {
  it('previews reach, path and sight from the server', async () => {
    const posted: string[] = []
    const { wrapper } = await mountApp('/gallery', {
      '/api/v1/rules/hex/reach': async (_u, req) => {
        const body = (await req.clone().json()) as { to?: unknown }
        posted.push(body.to ? 'reach+path' : 'reach')
        return {
          hexes: [
            { q: 0, r: 0, costFt: 0, canEnd: true, from: { q: 0, r: 0 } },
            { q: 1, r: 0, costFt: 5, canEnd: true, from: { q: 0, r: 0 } },
            { q: -2, r: 2, costFt: 10, canEnd: false, from: { q: -1, r: 1 } },
          ],
          ...(body.to ? { path: [{ q: 0, r: 0 }, { q: 1, r: 0 }], pathCostFt: 5 } : {}),
        }
      },
      '/api/v1/rules/hex/sight': () => ({ visible: true, cover: 'three_quarters', acBonus: 5 }),
    })
    const sandboxEl = () => wrapper.get('[data-testid="movement-sandbox"]')
    await sandboxEl().get('[data-hex="0,0"]').trigger('click')
    await flushPromises()
    expect(sandboxEl().get('[data-testid="reach-summary"]').text()).toBe('2 hexes reachable')
    expect(sandboxEl().get('[data-hex="1,0"]').classes()).toContain('hex--reach')
    await sandboxEl().get('input[value="look"]').setValue(true)
    await sandboxEl().get('[data-hex="1,0"]').trigger('click')
    await flushPromises()
    expect(sandboxEl().get('[data-testid="reach-summary"]').text()).toContain('path costs 5 ft')
    expect(sandboxEl().get('[data-testid="sight-summary"]').text()).toBe('Visible, three-quarters cover (+5 AC)')
    expect(sandboxEl().get('[data-hex="1,0"]').classes()).toContain('hex--path')
    expect(sandboxEl().get('[data-hex="0,-1"]').classes()).toContain('hex--wall')
    expect(sandboxEl().get('[data-hex="1,-3"]').classes()).toContain('hex--enemy')
    expect(posted).toEqual(['reach', 'reach+path'])
  })

  it('reports hidden targets and failures', async () => {
    const { wrapper } = await mountApp('/gallery', {
      '/api/v1/rules/hex/reach': () => ({ hexes: [] }),
      '/api/v1/rules/hex/sight': () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503),
    })
    const box = wrapper.get('[data-testid="movement-sandbox"]')
    await box.get('input[value="look"]').setValue(true)
    await box.get('[data-hex="3,0"]').trigger('click')
    await flushPromises()
    expect(box.get('[data-hex="3,0"]').classes()).toContain('hex--seen')
    expect(box.get('[role="alert"]').text()).toContain('could not be computed')
    document.body.innerHTML = ''
    const hidden = await mountApp('/gallery', {
      '/api/v1/rules/hex/reach': () => ({ hexes: [] }),
      '/api/v1/rules/hex/sight': () => ({ visible: false, cover: 'total', acBonus: 0 }),
    })
    const h = hidden.wrapper.get('[data-testid="movement-sandbox"]')
    await h.get('input[value="look"]').setValue(true)
    await h.get('[data-hex="2,-4"]').trigger('click')
    await flushPromises()
    expect(h.get('[data-testid="sight-summary"]').text()).toBe('Out of sight')
  })
})
