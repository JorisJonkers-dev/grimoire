import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { Vehicle, Vehicles } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const GULL = '0190c7a8-0000-7000-8000-0000000000a1'
const CART = '0190c7a8-0000-7000-8000-0000000000a2'
const WRECK = '0190c7a8-0000-7000-8000-0000000000a3'
const SAIL = '0190c7a8-0000-7000-8000-0000000000b1'
const WHEEL = '0190c7a8-0000-7000-8000-0000000000b2'
const HELM = '0190c7a8-0000-7000-8000-0000000000c1'
const RIGGING = '0190c7a8-0000-7000-8000-0000000000c2'
const gull: Vehicle = {
  id: GULL, name: 'The Gull', kind: 'water', hull: 260, hullMax: 300, threshold: 15, milesPerDay: 48, speed: 12, shortHanded: true,
  components: [{ id: SAIL, name: 'Mainsail', hp: 0, hpMax: 100, drives: true }, { id: WHEEL, name: 'Ship\'s wheel', hp: 50, hpMax: 50, drives: false }],
  stations: [{ id: HELM, name: 'Helm', crew: 1, posted: 1 }, { id: RIGGING, name: 'Rigging', crew: 6, posted: 4 }, { id: '0190c7a8-0000-7000-8000-0000000000c3', name: 'Lookout', crew: 2, posted: 0 }],
}
const cart: Vehicle = { id: CART, name: 'Cart', kind: 'land', hull: 20, hullMax: 20, threshold: 0, milesPerDay: 24, speed: 24, shortHanded: false, components: [], stations: [] }
const wreck: Vehicle = { ...cart, id: WRECK, name: 'Skyskiff', kind: 'air', hull: 0, speed: 0 }
const becalmed: Vehicle = { ...cart, id: '0190c7a8-0000-7000-8000-0000000000a4', name: 'Becalmed', kind: 'water', speed: 0 }
const seenByDM: Vehicles = { dm: true, vehicles: [gull, cart, wreck, becalmed] }
const seenByPlayer: Vehicles = { ...seenByDM, dm: false }

afterEach(() => { unmountAll() })

function nth<T>(list: T[], i: number): T {
  const item = list[i]
  if (item === undefined) throw new Error(`nothing at ${String(i)}`)
  return item
}

function backend(vehicles: Vehicles) {
  const calls: string[] = []
  const state = { fail: 0 }
  const failing = () => jsonResponse({ type: 'about:blank', title: 'No', status: state.fail }, state.fail)
  const routes = {
    [`/api/v1/campaigns/${ID}/vehicles/`]: async (url: URL, req: Request) => {
      calls.push(`${req.method} ${url.pathname.split('/').slice(6).join('/')} ${req.method === 'DELETE' ? '' : await req.text()}`.trim())
      if (state.fail) return failing()
      return req.method === 'DELETE' ? new Response(null, { status: 204 }) : jsonResponse(gull)
    },
    [`/api/v1/campaigns/${ID}/vehicles`]: async (_u: URL, req: Request) => {
      if (req.method === 'GET') return vehicles
      calls.push(`POST vehicles ${await req.text()}`)
      if (state.fail) return failing()
      return jsonResponse(cart, 201)
    },
  }
  return { calls, routes, state }
}

describe('vehicles', () => {
  it('shows a Player how each vehicle stands, and nothing to change it with', async () => {
    const { routes } = backend(seenByPlayer)
    const { wrapper } = await mountApp(`/campaigns/${ID}/vehicles`, routes)
    await flushPromises()
    const ship = wrapper.get(`[data-testid="vehicle-${GULL}"]`)
    expect(ship.get('h2').text()).toBe('The Gull')
    expect(ship.get('[data-testid="vehicle-stands"]').text()).toBe('Travels by water, round the clock. Makes 12 of its 48 miles a day: short of crew, it goes at half speed.')
    expect(ship.get('[data-testid="hull"]').text()).toBe('Hull: 260 of 300 hit points. Blows under 15 damage do nothing.')
    expect(ship.findAll('[data-testid="component"]').map((li) => li.text())).toEqual(['Mainsail, which drives it: broken', 'Ship\'s wheel: 50 of 50 hit points'])
    expect(ship.findAll('[data-testid="station"]').map((li) => li.text())).toEqual(['Helm: 1 of 1 crew', 'Rigging: 4 of 6 crew', 'Lookout: 0 of 2 crew'])
    const wagon = wrapper.get(`[data-testid="vehicle-${CART}"]`)
    expect(wagon.get('[data-testid="vehicle-stands"]').text()).toBe('Travels by land, eight hours a day. Makes 24 of its 24 miles a day.')
    expect(wagon.get('[data-testid="hull"]').text()).toBe('Hull: 20 of 20 hit points.')
    expect(wagon.find('[data-testid="component"]').exists()).toBe(false)
    expect(wagon.find('[data-testid="station"]').exists()).toBe(false)
    expect(wagon.findAll('ul')).toHaveLength(0)
    expect(wrapper.get(`[data-testid="vehicle-${WRECK}"] [data-testid="vehicle-stands"]`).text()).toBe('Travels by air, round the clock. A wreck: it goes nowhere.')
    expect(wrapper.get(`[data-testid="vehicle-${becalmed.id}"] [data-testid="vehicle-stands"]`).text()).toBe('Travels by water, round the clock. Nothing is left to drive it: it goes nowhere.')
    for (const id of ['vehicle-add', 'vehicle-remove', 'amount', 'hull-damage', 'hull-repair', 'part-damage', 'part-repair', 'crew-more', 'crew-fewer']) {
      expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    }
    await expectAccessible(wrapper.element as Element)
  })

  it('says so when there are none, and when the Campaign is not there', async () => {
    const { routes } = backend({ dm: false, vehicles: [] })
    const { wrapper } = await mountApp(`/campaigns/${ID}/vehicles`, routes)
    await flushPromises()
    expect(wrapper.get('[data-testid="no-vehicles"]').text()).toBe('No vehicles in this Campaign yet.')
    const missing = await mountApp(`/campaigns/${ID}/vehicles`, { [`/api/v1/campaigns/${ID}/vehicles`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404) })
    await flushPromises()
    expect(missing.wrapper.get('[data-testid="vehicles-missing"]').text()).toBe('That Campaign is not available.')
  })

  it('lets the DM damage and repair the hull and a component, post crew, and remove a vehicle', async () => {
    const { calls, routes, state } = backend(seenByDM)
    const { wrapper } = await mountApp(`/campaigns/${ID}/vehicles`, routes)
    await flushPromises()
    const ship = wrapper.get(`[data-testid="vehicle-${GULL}"]`)
    // Ten at a time unless told otherwise.
    await ship.get('[data-testid="hull-damage"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST ${GULL}/damage {"amount":10}`)
    await ship.get('[data-testid="amount"]').setValue(40)
    await ship.get('[data-testid="hull-repair"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST ${GULL}/damage {"amount":40,"repair":true}`)
    const sail = nth(ship.findAll('[data-testid="component"]'), 0)
    const wheel = nth(ship.findAll('[data-testid="component"]'), 1)
    await sail.get('[data-testid="part-repair"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST ${GULL}/damage {"amount":40,"componentId":"${SAIL}","repair":true}`)
    await wheel.get('[data-testid="part-damage"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST ${GULL}/damage {"amount":40,"componentId":"${WHEEL}"}`)
    expect(sail.get('[data-testid="part-damage"]').attributes('aria-label')).toBe('Damage Mainsail of The Gull')

    // Crew go on and off a station one at a time, never past what it takes or below none.
    const helm = nth(ship.findAll('[data-testid="station"]'), 0)
    const rigging = nth(ship.findAll('[data-testid="station"]'), 1)
    expect(helm.get('[data-testid="crew-more"]').attributes('disabled')).toBeDefined()
    expect(helm.get('[data-testid="crew-fewer"]').attributes('disabled')).toBeUndefined()
    const lookout = nth(ship.findAll('[data-testid="station"]'), 2)
    expect(lookout.get('[data-testid="crew-fewer"]').attributes('disabled')).toBeDefined()
    expect(lookout.get('[data-testid="crew-more"]').attributes('disabled')).toBeUndefined()
    await rigging.get('[data-testid="crew-more"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`PUT ${GULL}/stations/${RIGGING} {"posted":5}`)
    await rigging.get('[data-testid="crew-fewer"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`PUT ${GULL}/stations/${RIGGING} {"posted":3}`)
    await expectAccessible(wrapper.element as Element)

    // A failure says so; the next thing that works takes the message away.
    state.fail = 422
    await ship.get('[data-testid="hull-damage"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="vehicles-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    state.fail = 0
    await wrapper.get(`[data-testid="vehicle-${CART}"] [data-testid="vehicle-remove"]`).trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`DELETE ${CART}`)
    expect(wrapper.find('[data-testid="vehicles-problem"]').exists()).toBe(false)
    state.fail = 500
    await ship.get('[data-testid="vehicle-remove"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="vehicles-problem"]').exists()).toBe(true)
    await rigging.get('[data-testid="crew-more"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="vehicles-problem"]').exists()).toBe(true)
  })

  it('lets the DM build a vehicle with its components and crew stations', async () => {
    const { calls, routes, state } = backend(seenByDM)
    const { wrapper } = await mountApp(`/campaigns/${ID}/vehicles`, routes)
    await flushPromises()
    const add = wrapper.get('[data-testid="vehicle-add"]')
    const set = (id: string, value: string | number | boolean) => add.get(`[data-testid="${id}"]`).setValue(value)
    expect(add.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await set('vehicle-name', '  Dawn Treader ')
    expect(add.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    // As it comes: a land vehicle with nothing on it.
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST vehicles {"name":"Dawn Treader","kind":"land","hullMax":50,"threshold":0,"milesPerDay":24,"components":[],"stations":[]}')
    // The form starts afresh once a vehicle is built.
    expect((add.get('[data-testid="vehicle-name"]').element as HTMLInputElement).value).toBe('')

    await set('vehicle-name', 'Dawn Treader')
    await set('vehicle-kind', 'water')
    await set('vehicle-hull', 200)
    await set('vehicle-threshold', 10)
    await set('vehicle-speed', 72)
    await add.get('[data-testid="component-add"]').trigger('click')
    await add.get('[data-testid="component-add"]').trigger('click')
    await add.get('[data-testid="component-add"]').trigger('click')
    await set('component-0-name', ' Sail ')
    await set('component-0-hp', 80)
    await set('component-0-drives', true)
    await set('component-1-name', 'Figurehead')
    await add.get('[data-testid="component-2-remove"]').trigger('click')
    await add.get('[data-testid="station-add"]').trigger('click')
    await add.get('[data-testid="station-add"]').trigger('click')
    await set('station-0-name', ' Oars ')
    await set('station-0-crew', 12)
    await add.get('[data-testid="station-1-remove"]').trigger('click')
    await expectAccessible(wrapper.element as Element)
    state.fail = 422
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(
      'POST vehicles {"name":"Dawn Treader","kind":"water","hullMax":200,"threshold":10,"milesPerDay":72,"components":[{"name":"Sail","hpMax":80,"drives":true},{"name":"Figurehead","hpMax":10,"drives":false}],"stations":[{"name":"Oars","crew":12}]}',
    )
    // Refused, the form keeps what was entered.
    expect(wrapper.find('[data-testid="vehicles-problem"]').exists()).toBe(true)
    expect((add.get('[data-testid="vehicle-name"]').element as HTMLInputElement).value).toBe('Dawn Treader')
  })
})
