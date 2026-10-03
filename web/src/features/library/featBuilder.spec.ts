import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { BackgroundBuild, BackgroundDesign, FeatBuild, FeatDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const FEAT = '0190c7a8-0000-7000-8000-0000000000e6'
const BACK = '0190c7a8-0000-7000-8000-0000000000e7'
const at = '2026-10-03T20:00:00Z'
const featStart: FeatDesign = { category: 'general', text: '', prerequisites: [] }
const backStart: BackgroundDesign = { abilities: ['strength', 'dexterity', 'constitution'], skills: ['athletics', 'survival'], feat: 'alert', featName: 'Alert', gold: 50 }
const featEntry = { id: FEAT, kind: 'feat' as const, name: 'Lampwright', fields: [], revision: 1, createdAt: at, updatedAt: at }
const backEntry = { id: BACK, kind: 'background' as const, name: 'Bogwarden', fields: [], revision: 1, createdAt: at, updatedAt: at }
const featBuild = (design: FeatDesign, extra: Partial<FeatBuild> = {}): FeatBuild => ({ entry: featEntry, design, slug: 'hb-f', lines: ['General feat'], ...extra })
const backBuild = (design: BackgroundDesign, extra: Partial<BackgroundBuild> = {}): BackgroundBuild => ({ entry: backEntry, design, slug: 'hb-b', lines: ['Ability Scores: Strength'], ...extra })

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('feat builder', () => {
  it('builds the Lampwright with prerequisites, previews and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${FEAT}/feat`, {
      '/api/v1/builders/feats/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: FeatDesign }
        sent.push({ method: 'PREVIEW', body })
        return featBuild(body.design, { lines: ['General feat, repeatable', 'Prerequisites: Level 4+'] })
      },
      [`/api/v1/builders/feats/${FEAT}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as FeatDesign
          sent.push({ method: 'PUT', body })
          return featBuild(body, { entry: { ...featEntry, revision: 2 } })
        }
        return featBuild(structuredClone(featStart))
      },
    })
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    await set('feat-category', 'epic_boon')
    await set('feat-repeatable', true)
    await set('feat-text', 'Your lanterns burn twice as long.')
    for (let i = 0; i < 4; i++) await wrapper.get('[data-testid="feat-add-pre"]').trigger('click')
    await set('feat-pre-0-minimum', 4)
    await set('feat-pre-1-kind', 'ability')
    await set('feat-pre-1-ability', 'wisdom')
    await set('feat-pre-1-minimum', 13)
    await set('feat-pre-2-kind', 'feat')
    await set('feat-pre-2-feat', 'alert')
    await set('feat-pre-3-kind', 'spellcasting')
    await set('feat-pre-3-group', 3)
    await wrapper.get('[data-testid="feat-add-pre"]').trigger('click')
    await wrapper.get('[data-testid="feat-pre-4-remove"]').trigger('click')
    await wrapper.get('[data-testid="feat-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="feat-lines"]').text()).toContain('Prerequisites: Level 4+')
    expect((sent[0]?.body as { name: string }).name).toBe('Lampwright')
    await wrapper.get('[data-testid="feat-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="feat-status"]').text()).toBe('Saved as Revision 2.')
    expect(sent.find((x) => x.method === 'PUT')?.body).toEqual({
      category: 'epic_boon', repeatable: true, text: 'Your lanterns burn twice as long.',
      prerequisites: [
        { kind: 'level', minimum: 4, group: 0 },
        { kind: 'ability', ability: 'wisdom', minimum: 13, group: 1 },
        { kind: 'feat', minimum: 4, feat: 'alert', group: 2 },
        { kind: 'spellcasting', minimum: 4, group: 3 },
      ],
    })
  })

  it('shows a Shared Library copy, reports a design that will not build, and opens from the entry', async () => {
    const { wrapper } = await mountApp(`/library/${FEAT}/feat`, {
      '/api/v1/builders/feats/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'choose a category' }, 422),
      [`/api/v1/builders/feats/${FEAT}`]: () => featBuild(featStart, { entry: { ...featEntry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="feat-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="feat-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="feat-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="feat-problem"]').text()).toBe('choose a category')
    const refused = await mountApp(`/library/${FEAT}/feat`, {
      [`/api/v1/builders/feats/${FEAT}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="feat-error"]').text()).toContain('cannot be opened')
    const fromEntry = await mountApp(`/library/${FEAT}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/feats/${FEAT}`]: () => featBuild(featStart),
      [`/api/v1/library/${FEAT}`]: () => ({ entry: featEntry, revisions: [], uses: [] }),
    })
    await fromEntry.wrapper.get('[data-testid="open-feat-builder"]').trigger('click')
    await flushPromises()
    expect(fromEntry.wrapper.get('h1').text()).toContain('Feat builder')
  })
})

describe('background builder', () => {
  it('builds the Bogwarden, previews and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${BACK}/background`, {
      '/api/v1/builders/backgrounds/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: BackgroundDesign }
        sent.push({ method: 'PREVIEW', body })
        return backBuild(body.design, { lines: ['Skill Proficiencies: Survival, Nature'] })
      },
      [`/api/v1/builders/backgrounds/${BACK}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as BackgroundDesign
          sent.push({ method: 'PUT', body })
          return backBuild(body, { entry: { ...backEntry, revision: 2 } })
        }
        return backBuild(structuredClone(backStart))
      },
    })
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    await set('background-ability-1', 'wisdom')
    await set('background-skill-0', 'nature')
    await set('background-feat-name', 'Lucky')
    await set('background-feat', 'lucky')
    await set('background-tool', 'Herbalism Kit')
    await set('background-gold', 40)
    await set('background-equipment', 'A lantern and a pole')
    await set('background-text', 'You kept the causeways open.')
    await wrapper.get('[data-testid="background-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="background-lines"]').text()).toContain('Survival, Nature')
    expect((sent[0]?.body as { name: string }).name).toBe('Bogwarden')
    await wrapper.get('[data-testid="background-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="background-status"]').text()).toBe('Saved as Revision 2.')
    expect(sent.find((x) => x.method === 'PUT')?.body).toEqual({
      abilities: ['strength', 'wisdom', 'constitution'], skills: ['nature', 'survival'], feat: 'lucky', featName: 'Lucky', tool: 'Herbalism Kit', gold: 40,
      equipment: 'A lantern and a pole', text: 'You kept the causeways open.',
    })
  })

  it('shows a Shared Library copy, reports a design that will not build, and opens from the entry', async () => {
    const { wrapper } = await mountApp(`/library/${BACK}/background`, {
      '/api/v1/builders/backgrounds/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'grant two different skills' }, 422),
      [`/api/v1/builders/backgrounds/${BACK}`]: () => backBuild(backStart, { entry: { ...backEntry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="background-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="background-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="background-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="background-problem"]').text()).toBe('grant two different skills')
    const refused = await mountApp(`/library/${BACK}/background`, {
      [`/api/v1/builders/backgrounds/${BACK}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="background-error"]').text()).toContain('cannot be opened')
    const fromEntry = await mountApp(`/library/${BACK}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/backgrounds/${BACK}`]: () => backBuild(backStart),
      [`/api/v1/library/${BACK}`]: () => ({ entry: backEntry, revisions: [], uses: [] }),
    })
    await fromEntry.wrapper.get('[data-testid="open-background-builder"]').trigger('click')
    await flushPromises()
    expect(fromEntry.wrapper.get('h1').text()).toContain('Background builder')
  })
})
