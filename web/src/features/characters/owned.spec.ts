import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { sheet } from '@/test/sheet'

const me = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'tamsin', nickname: 'Tamsin', email: 't@example.com',
  admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false,
}
const morvain = '0190c7a8-0000-7000-8000-0000000000a1'
const saltmarsh = '0190c7a8-0000-7000-8000-0000000000a2'
const kara = {
  id: '0190c7a8-0000-7000-8000-0000000000b1', name: 'Kara', ruleset: 'srd-2024', species: 'human', class: 'fighter', background: 'soldier',
  backstory: 'Raised by wolves.', hasPortrait: false,
  campaigns: [{ campaignId: morvain, campaignName: 'Morvain', characterId: '0190c7a8-0000-7000-8000-0000000000e1', level: 3, hpCurrent: 5, hpMax: 28 }],
}
const campaigns = { items: [
  { id: morvain, name: 'Morvain', ruleset: 'srd-2024', myRole: 'player', memberCount: 3, createdAt: '2026-09-01T10:00:00Z' },
  { id: saltmarsh, name: 'Saltmarsh', ruleset: 'srd-2024', myRole: 'player', memberCount: 2, createdAt: '2026-09-01T10:00:00Z' },
] }

afterEach(() => { unmountAll() })

describe('your Characters', () => {
  it('lists each Character with the Campaigns it plays in, from the navbar', async () => {
    const { wrapper } = await mountApp('/characters', {
      '/api/v1/characters': () => ({ items: [kara, { ...kara, id: '0190c7a8-0000-7000-8000-0000000000b2', name: 'Odile', campaigns: [] }] }),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="characters-link"]').attributes('href')).toBe('/characters')
    const list = wrapper.get('[data-testid="my-characters"]').text()
    expect(list).toContain('Morvain · level 3')
    expect(list).toContain('Not in a Campaign yet')
  })

  it('says when there are none', async () => {
    const { wrapper } = await mountApp('/characters', { '/api/v1/characters': () => ({ items: [] }), '/api/v1/account': () => me })
    expect(wrapper.get('[data-testid="my-characters-none"]').text()).toContain('No Characters yet')
  })

  it('edits the identity and brings the Character into another Campaign', async () => {
    const sent: unknown[] = []
    let joins = 0
    const { wrapper, router } = await mountApp(`/characters/${kara.id}`, {
      [`/api/v1/characters/${kara.id}/campaigns`]: async (_u, req) => {
        sent.push(await req.json())
        joins += 1
        return joins === 1 ? jsonResponse({ status: 422, title: 'No' }, 422) : jsonResponse(sheet({ id: '0190c7a8-0000-7000-8000-0000000000e2' }), 201)
      },
      [`/api/v1/characters/${kara.id}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          sent.push(await req.json())
          return { ...kara, name: 'Kara Vale' }
        }
        return kara
      },
      '/api/v1/campaigns/': () => ({ id: saltmarsh }),
      '/api/v1/campaigns': () => campaigns,
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="my-character-campaigns"]').text()).toContain('5/28 HP')
    await wrapper.get('[data-testid="my-character-name"]').setValue(' Kara Vale ')
    await wrapper.get('[data-testid="my-character-form"]').trigger('submit')
    await flushPromises()
    expect(sent[0]).toEqual({ name: 'Kara Vale', backstory: 'Raised by wolves.' })
    expect(wrapper.get('[data-testid="my-character-saved"]').text()).toContain('every Campaign')
    const options = wrapper.get('[data-testid="my-character-target"]').findAll('option').map((o) => o.text())
    expect(options).toEqual(['Choose a Campaign', 'Saltmarsh'])
    await wrapper.get('[data-testid="my-character-target"]').setValue(saltmarsh)
    await wrapper.get('[data-testid="my-character-join"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="my-character-join-failed"]').text()).toContain('rules do not allow')
    await wrapper.get('[data-testid="my-character-join"]').trigger('submit')
    await flushPromises()
    expect(sent[2]).toEqual({ campaignId: saltmarsh })
    await vi.waitFor(() => { expect(router.currentRoute.value.fullPath).toBe(`/campaigns/${saltmarsh}/characters/0190c7a8-0000-7000-8000-0000000000e2`) })
  })

  it('says when the Character is not yours', async () => {
    const { wrapper } = await mountApp(`/characters/${kara.id}`, {
      '/api/v1/characters': () => jsonResponse({ status: 404, title: 'Not found' }, 404),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="my-character-error"]').text()).toContain('could not be read')
  })
})
