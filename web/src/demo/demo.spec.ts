import { afterEach, describe, expect, it, vi } from 'vitest'
import { createAppRouter } from '@/app/router'
import { mountDemo } from '@/test/mountDemo'
import { demoPages } from './pages'
import { personas } from './role'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('the demo page list', () => {
  it('names every route of the app', () => {
    const routes = createAppRouter().getRoutes().map((r) => r.name).filter((n): n is string => typeof n === 'string')
    expect(demoPages.map((p) => p.name).sort()).toEqual([...routes].sort())
  })
})

const cases = demoPages.flatMap((page) => page.personas.map((persona) => ({ page: page.name, path: page.path, persona })))

describe('every page renders in the demo for each caller it serves', () => {
  it.each(cases)('$page as $persona', async ({ page, path, persona }) => {
    const { router, demo, network, wrapper } = await mountDemo(path, persona)
    expect(router.currentRoute.value.name).toBe(page)
    expect(demo.unhandled).toEqual([])
    // Signed out, the header still asks who is signed in and is told nobody is, as in production.
    const refused = demo.answers.filter((a) => a.status >= 400 && !(persona === 'signed-out' && a.what.startsWith('getAccount ') && a.status === 401))
    expect(refused).toEqual([])
    expect(network).not.toHaveBeenCalled()
    expect(wrapper.findAll('[role="alert"]').map((a) => a.text())).toEqual([])
  })
})

const guarded = demoPages.filter((p) => !p.personas.includes('signed-out'))

describe('signed out, a page that needs an Account sends the caller to sign in', () => {
  it.each(guarded.map((p) => ({ page: p.name, path: p.path })))('$page', async ({ path }) => {
    const { router, network } = await mountDemo(path, 'signed-out')
    expect(router.currentRoute.value.name).toBe('sign-in')
    expect(network).not.toHaveBeenCalled()
  })
})

describe('the role switcher', () => {
  it('offers every persona', async () => {
    const { wrapper } = await mountDemo('/compendium/spells', 'player')
    expect(personas.map((p) => wrapper.find(`[data-testid="demo-as-${p.id}"]`).exists())).toEqual(personas.map(() => true))
  })

  it('looks at the same page again as the persona chosen', async () => {
    const { wrapper, demo, settle } = await mountDemo('/campaigns', 'player')
    await wrapper.find('[data-testid="demo-as-admin"]').setValue(true)
    await settle()
    expect(demo.world.persona).toBe('admin')
    expect(wrapper.find('[data-testid="account-menu-toggle"]').attributes('aria-label')).toContain('Admin')
  })
})
