import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const CH = '0190c7a8-0000-7000-8000-000000000009'
const base = `/api/v1/campaigns/${ID}/characters/${CH}`
const sp = (slug: string, name: string, level = 1, ritual = false) => ({ slug, name, level, ritual })

const wizard = (extra = {}) => ({
  class: 'wizard', name: 'Wizard', level: 1, limit: 2, maxLevel: 1, keepsSpellbook: true, allotment: 6,
  cantrips: [sp('light', 'Light', 0)], always: [],
  prepared: [sp('shield', 'Shield')],
  spellbook: [sp('shield', 'Shield'), sp('sleep', 'Sleep'), sp('find-familiar', 'Find Familiar', 1, true)],
  options: [sp('shield', 'Shield'), sp('sleep', 'Sleep'), sp('find-familiar', 'Find Familiar', 1, true)],
  copyable: [sp('alarm', 'Alarm', 1, true), sp('magic-missile', 'Magic Missile')],
  ...extra,
})
const druid = {
  class: 'druid', name: 'Druid', level: 1, limit: 4, maxLevel: 1, keepsSpellbook: false, allotment: 0,
  cantrips: [], always: [sp('speak-with-animals', 'Speak with Animals', 1, true)], prepared: [sp('entangle', 'Entangle')],
  spellbook: [], options: [sp('entangle', 'Entangle')], copyable: [],
}
const casting = (extra = {}) => ({ canPrepare: true, classes: [wizard()], purse: [{ coin: 'gp', count: 60 }], clock: { day: 2, minute: 125 }, ...extra })

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('spells page', () => {
  it('prepares spells from the spellbook, casts a ritual and copies a spell', async () => {
    const sent: { url: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/spells`, {
      [`${base}/spells/rituals`]: async (u, req) => {
        sent.push({ url: u.pathname, body: await req.clone().json() })
        return { spell: sp('find-familiar', 'Find Familiar', 1, true), minutes: 70, clock: { day: 2, minute: 195 } }
      },
      [`${base}/spells/prepared`]: async (u, req) => {
        sent.push({ url: u.pathname, body: await req.clone().json() })
        return casting()
      },
      [`${base}/spellbook`]: async (u, req) => {
        sent.push({ url: u.pathname, body: await req.clone().json() })
        return casting()
      },
      [`${base}/spells`]: () => casting(),
    })
    expect(wrapper.get('[data-testid="spells-meta"]').text()).toBe('Day 2, 02:05 · 60 gp')
    await expectAccessible(wrapper.element as Element)
    const prep = wrapper.get('[data-testid="prepare-wizard"]')
    expect(prep.text()).toContain('Prepared · 1 of 2')
    await prep.get('[data-testid="prep-sleep"]').trigger('change')
    await prep.get('[data-testid="prep-find-familiar"]').trigger('change')
    expect(prep.text()).toContain('Prepared · 2 of 2')
    await prep.get('[data-testid="prep-shield"]').trigger('change')
    await prep.get('[data-testid="prepare-save-wizard"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="spells-status"]').text()).toBe('Wizard spells prepared.')
    await wrapper.get('[data-testid="ritual-find-familiar"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="spells-status"]').text()).toContain('70 minutes. It is now Day 2, 03:15.')
    expect(wrapper.get('[data-testid="copy-cost"]').text()).toBe('Free: 3 of 6 left for this level.')
    await wrapper.get('[data-testid="copy-spell"]').setValue('alarm')
    await wrapper.get('[data-testid="copy-submit"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="spells-status"]').text()).toBe('Copied into the spellbook.')
    expect(sent).toEqual([
      { url: `${base}/spells/prepared`, body: { class: 'wizard', spells: ['sleep'] } },
      { url: `${base}/spells/rituals`, body: { spell: 'find-familiar' } },
      { url: `${base}/spellbook`, body: { spell: 'alarm' } },
    ])
  })

  it('shows the cost once the book is full, locked preparation, always-prepared spells and failures', async () => {
    const full = wizard({ allotment: 3 })
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/spells`, {
      [`${base}/spellbook`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'copying Magic Missile costs 50 gold pieces' }, 422),
      [`${base}/spells`]: () => casting({ canPrepare: false, purse: [], classes: [full, druid] }),
    })
    expect(wrapper.get('[data-testid="spells-meta"]').text()).toContain('No coins')
    expect(wrapper.find('[data-testid="prepare-wizard"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="spells-druid"]').get('[data-testid="always"]').text()).toContain('Speak with Animals')
    expect(wrapper.get('[data-testid="copy-cost"]').text()).toBe('Costs 50 gp and 2 hours.')
    await wrapper.get('[data-testid="copy-spell"]').setValue('magic-missile')
    await wrapper.get('[data-testid="copy-submit"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="spells-problem"]').text()).toBe('copying Magic Missile costs 50 gold pieces')
    await expectAccessible(wrapper.element as Element)
  })

  it('says when a Character casts nothing, or the page will not load', async () => {
    const none = await mountApp(`/campaigns/${ID}/characters/${CH}/spells`, {
      [`${base}/spells`]: () => casting({ classes: [] }),
    })
    expect(none.wrapper.find('[data-testid="no-spells"]').exists()).toBe(true)
    unmountAll()
    const empty = await mountApp(`/campaigns/${ID}/characters/${CH}/spells`, {
      [`${base}/spells`]: () => casting({ classes: [wizard({ options: [], spellbook: [], prepared: [] })] }),
    })
    expect(empty.wrapper.text()).toContain('Copy spells into your spellbook first.')
    unmountAll()
    const plain = await mountApp(`/campaigns/${ID}/characters/${CH}/spells`, {
      [`${base}/spells`]: () => casting({ classes: [{ ...druid, options: [] }] }),
    })
    expect(plain.wrapper.text()).toContain('Nothing to prepare yet.')
    unmountAll()
    const broken = await mountApp(`/campaigns/${ID}/characters/${CH}/spells`, {})
    expect(broken.wrapper.find('[data-testid="spells-error"]').exists()).toBe(true)
  })
})
