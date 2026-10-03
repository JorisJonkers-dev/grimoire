import { flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { cellsBetween, squareLines } from './geometry'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const MID = '0190c7a8-0000-7000-8000-000000000021'
const realm = {
  id: MID, name: 'Realm', kind: 'world' as const, imageUrl: `/api/v1/campaigns/${ID}/maps/${MID}/image`, width: 400, height: 300,
  hexSizePx: 40, originX: 34.64, originY: 40, ambient: 'bright', gridKind: 'hexes', gridStrength: 20, scaleMiles: 6,
}
const crypt = { ...realm, name: 'Crypt', kind: 'local' as const }
const problem = () => jsonResponse({ type: 'about:blank', title: 'x', status: 422 }, 422)

async function open(map: typeof realm | typeof crypt, calibrated: (body: Record<string, number>) => unknown = () => map) {
  const writes: string[] = []
  const mounted = await mountApp(`/campaigns/${ID}/maps/${MID}`, {
    [`/api/v1/campaigns/${ID}/maps/${MID}/calibration`]: async (_u, req) => {
      const body = (await req.json()) as Record<string, number>
      writes.push(`CALIBRATE ${JSON.stringify(body)}`)
      return calibrated(body)
    },
    [`/api/v1/campaigns/${ID}/maps/${MID}`]: async (_u, req) => {
      if (req.method === 'PUT') writes.push(`PUT ${JSON.stringify(await req.json())}`)
      return map
    },
  })
  return { ...mounted, writes }
}

describe('map geometry', () => {
  it('measures in cells and rules squares as wide as the hexes they stand for', () => {
    const across = 40 * Math.sqrt(3)
    expect(cellsBetween(40, { x: 10, y: 10 }, { x: 10 + 3 * across, y: 10 })).toBeCloseTo(3)
    expect(cellsBetween(40, { x: 0, y: 0 }, { x: 3 * across, y: 4 * across })).toBeCloseTo(5)
    expect(cellsBetween(40, { x: 7, y: 7 }, { x: 7, y: 7 })).toBe(0)
    // A square is centred on the grid's origin: lines run half a cell either side of it, across the whole picture.
    const d = squareLines({ size: 20 / Math.sqrt(3), origin: { x: 10, y: 10 } }, 50, 30)
    expect(d).toBe('M0 0V30 M20 0V30 M40 0V30 M0 0H50 M0 20H50')
    expect(squareLines({ size: 20 / Math.sqrt(3), origin: { x: 25, y: 4 } }, 40, 20)).toBe('M15 0V20 M35 0V20 M0 14H40')
  })
})

describe('world map scale', () => {
  it('draws the grid see-through over the picture, as hexes, squares or not at all', async () => {
    const { wrapper } = await open(realm)
    const board = () => wrapper.get('[data-testid="map-board"]')
    expect(board().attributes('data-grid')).toBe('hexes')
    expect(board().attributes('style')).toContain('--grid: 0.2')
    expect(board().attributes('style')).toContain('--grid-squares: 0;')
    expect(wrapper.find('[data-testid="grid-squares"]').exists()).toBe(false)
    // The picture lies under the grid, whole.
    expect(wrapper.get('[data-testid="map-image"]').attributes('href')).toBe(realm.imageUrl)

    await wrapper.get('[data-testid="grid-strength"]').setValue(65)
    expect(board().attributes('style')).toContain('--grid: 0.65')
    expect(wrapper.get('[data-testid="grid-strength-value"]').text()).toBe('65%')
    await wrapper.get('[data-testid="grid-kind"]').setValue('squares')
    expect(board().attributes('data-grid')).toBe('squares')
    expect(board().attributes('style')).toContain('--grid: 0;')
    expect(board().attributes('style')).toContain('--grid-squares: 0.65')
    expect(wrapper.get('[data-testid="grid-squares"]').attributes('d')).toMatch(/^M[\d.]+ 0V300 /)
    await wrapper.get('[data-testid="grid-kind"]').setValue('off')
    expect(board().attributes('data-grid')).toBe('off')
    expect(board().attributes('style')).toContain('--grid: 0;')
    expect(wrapper.find('[data-testid="grid-squares"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
  })

  it('saves the grid kind, the miles a cell covers and the strength', async () => {
    const { wrapper, writes } = await open(realm)
    expect((wrapper.get('[data-testid="scale-miles"]').element as HTMLInputElement).value).toBe('6')
    await wrapper.get('[data-testid="grid-kind"]').setValue('squares')
    await wrapper.get('[data-testid="scale-miles"]').setValue(12)
    await wrapper.get('[data-testid="grid-strength"]').setValue(45)
    await wrapper.get('[data-testid="map-calibrate"]').trigger('submit')
    await flushPromises()
    expect(writes).toEqual(['PUT {"name":"Realm","hexSizePx":40,"originX":34.64,"originY":40,"ambient":"bright","gridStrength":45,"gridKind":"squares","scaleMiles":12}'])
    expect(wrapper.get('[data-testid="calibration-saved"]').text()).toBe('Saved.')
  })

  it('calibrates by dragging two points onto a known distance', async () => {
    const { wrapper, writes } = await open(realm, (body) => ({ ...realm, hexSizePx: 20, originX: body.ax, originY: body.ay }))
    const a = wrapper.get('[data-testid="calibrate-a"]')
    const b = wrapper.get('[data-testid="calibrate-b"]')
    const at = (h: typeof a) => `${String(h.attributes('cx'))},${String(h.attributes('cy'))}`
    // The points start a third and two thirds of the way across the picture.
    expect([at(a), at(b)]).toEqual(['133,150', '267,150'])
    const measured = () => wrapper.get('[data-testid="calibrate-measured"]').text()
    expect(measured()).toBe('Now 1.9 cells, 11.6 miles apart.')

    // Dragging follows the pointer and stays on the picture; letting go ends the drag.
    a.element.dispatchEvent(new MouseEvent('pointerdown', { clientX: 133, clientY: 150, bubbles: true }))
    window.dispatchEvent(new MouseEvent('pointermove', { clientX: 60, clientY: 80 }))
    await flushPromises()
    expect(at(a)).toBe('60,80')
    window.dispatchEvent(new MouseEvent('pointermove', { clientX: -30, clientY: 900 }))
    await flushPromises()
    expect(at(a)).toBe('0,300')
    window.dispatchEvent(new MouseEvent('pointermove', { clientX: 20, clientY: 30 }))
    window.dispatchEvent(new MouseEvent('pointerup'))
    window.dispatchEvent(new MouseEvent('pointermove', { clientX: 300, clientY: 300 }))
    await flushPromises()
    expect(at(a)).toBe('20,30')
    b.element.dispatchEvent(new MouseEvent('pointerdown', { clientX: 267, clientY: 150, bubbles: true }))
    window.dispatchEvent(new MouseEvent('pointermove', { clientX: 500, clientY: 30 }))
    window.dispatchEvent(new MouseEvent('pointerup'))
    await flushPromises()
    expect([at(a), at(b)]).toEqual(['20,30', '400,30'])
    // The keyboard moves a point a pixel at a time, or ten with Shift.
    await b.trigger('keydown', { key: 'ArrowLeft', shiftKey: true })
    await b.trigger('keydown', { key: 'ArrowLeft' })
    await b.trigger('keydown', { key: 'ArrowDown' })
    await b.trigger('keydown', { key: 'ArrowUp', shiftKey: true })
    await b.trigger('keydown', { key: 'ArrowRight' })
    await b.trigger('keydown', { key: 'Enter' })
    expect(at(b)).toBe('390,21')
    expect(wrapper.get('[data-testid="calibrate-line"]').attributes()).toMatchObject({ x1: '20', y1: '30', x2: '390', y2: '21' })
    expect(a.attributes('aria-label')).toBe('First point, at 20, 30. Drag it, or move it with the arrow keys.')

    expect(wrapper.get('[data-testid="calibrate-go"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="calibrate-distance"]').setValue(120)
    await wrapper.get('[data-testid="calibrate-go"]').trigger('click')
    await flushPromises()
    expect(writes).toEqual(['CALIBRATE {"ax":20,"ay":30,"bx":390,"by":21,"distance":120}'])
    // The form takes the grid the server worked out, and the measure agrees with what was said.
    expect((wrapper.get('[data-testid="hex-size"]').element as HTMLInputElement).value).toBe('20')
    expect((wrapper.get('[data-testid="origin-x"]').element as HTMLInputElement).value).toBe('20')
    expect(wrapper.get('[data-testid="calibrated"]').text()).toBe('Calibrated: a cell is 34.6 px across.')
    expect(measured()).toBe('Now 10.7 cells, 64.1 miles apart.')
    await expectAccessible(wrapper.element as Element)
  })

  it('says when two points cannot calibrate the map', async () => {
    const { wrapper } = await open(realm, problem)
    await wrapper.get('[data-testid="calibrate-distance"]').setValue(5)
    await wrapper.get('[data-testid="calibrate-go"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="calibrate-failed"]').text()).toBe('Those points and that distance make cells too small or too large. Move the points or change the distance.')
    expect(wrapper.find('[data-testid="calibrated"]').exists()).toBe(false)
  })
})

describe('battle map scale', () => {
  it('keeps its 5 ft hexes: only the strength of the grid changes, and distances are in feet', async () => {
    const { wrapper, writes } = await open(crypt, (body) => ({ ...crypt, hexSizePx: 30, originX: body.ax, originY: body.ay }))
    expect(wrapper.find('[data-testid="grid-kind"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="scale-miles"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="scale-fixed"]').text()).toBe('A battle map keeps its hexes: each is 5 feet across.')
    expect(wrapper.get('[data-testid="calibrate-measured"]').text()).toBe('Now 1.9 cells, 9.7 feet apart.')
    await wrapper.get('[data-testid="grid-strength"]').setValue(0)
    expect(wrapper.get('[data-testid="map-board"]').attributes('style')).toContain('--grid: 0;')
    await wrapper.get('[data-testid="map-calibrate"]').trigger('submit')
    await wrapper.get('[data-testid="calibrate-distance"]').setValue(25)
    await wrapper.get('[data-testid="calibrate-go"]').trigger('click')
    await flushPromises()
    expect(writes).toEqual([
      'PUT {"name":"Crypt","hexSizePx":40,"originX":34.64,"originY":40,"ambient":"bright","gridStrength":0}',
      'CALIBRATE {"ax":133,"ay":150,"bx":267,"by":150,"distance":25}',
    ])
    expect((wrapper.get('[data-testid="hex-size"]').element as HTMLInputElement).value).toBe('30')
  })
})

describe('the Default World', () => {
  it('becomes a world map of the Campaign at a click', async () => {
    const writes: string[] = []
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/maps`, {
      [`/api/v1/campaigns/${ID}/default-world`]: (_u, req) => {
        writes.push(req.method)
        return jsonResponse({ ...realm, name: 'Default World' }, 201)
      },
      [`/api/v1/campaigns/${ID}/maps/${MID}`]: () => ({ ...realm, name: 'Default World' }),
      [`/api/v1/campaigns/${ID}/maps`]: () => [],
    })
    await wrapper.get('[data-testid="use-default-world"]').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('map') }, { timeout: 5000 })
    await flushPromises()
    expect(writes).toEqual(['POST'])
    expect(router.currentRoute.value.params.mapId).toBe(MID)
    expect(wrapper.get('h1').text()).toBe('Default World')
  })

  it('says when it could not be added', async () => {
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/maps`, {
      [`/api/v1/campaigns/${ID}/default-world`]: problem,
      [`/api/v1/campaigns/${ID}/maps`]: () => [],
    })
    await wrapper.get('[data-testid="use-default-world"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="default-world-failed"]').text()).toBe('The Default World could not be added.')
    expect(router.currentRoute.value.name).toBe('maps')
    await expectAccessible(wrapper.element as Element)
  })
})
