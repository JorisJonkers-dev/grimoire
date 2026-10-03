import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { Companion } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [{ ...dm, isMe: as === 'dm' }, { ...player, isMe: as === 'player' }],
})
const companion = (n: number, name: string, extra: Partial<Companion> = {}): Companion => ({
  id: `0190c7a8-0000-7000-8000-0000000000d${String(n)}`, name, kind: 'companion', monsterSlug: 'wolf', sharesXp: false, notes: '', updatedAt: '2026-10-03T10:00:00Z', ...extra,
})

afterEach(() => { unmountAll() })

function backend(as: 'dm' | 'player', list: Companion[], fail = 0) {
  const calls: string[] = []
  const routes = {
    [`/api/v1/campaigns/${ID}/companions/`]: async (url: URL, req: Request) => {
      calls.push(`${req.method} ${url.pathname.slice(-2)} ${req.method === 'DELETE' ? '' : await req.text()}`.trim())
      if (fail) return jsonResponse({ status: fail, title: 'No', detail: 'There is no such creature in this Campaign.' }, fail)
      return req.method === 'DELETE' ? new Response(null, { status: 204 }) : jsonResponse(list[0])
    },
    [`/api/v1/campaigns/${ID}/companions`]: async (_u: URL, req: Request) => {
      if (req.method === 'GET') return list
      calls.push(`POST ${await req.text()}`)
      if (fail) return jsonResponse({ status: fail, title: 'Not allowed by the rules', detail: 'there is no such creature in this Campaign' }, fail)
      return jsonResponse(companion(9, 'New'), 201)
    },
    [`/api/v1/campaigns/${ID}/sessions`]: () => [],
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  }
  return { calls, routes }
}

describe('Companions on the Campaign page', () => {
  it('lets the DM add an ally, hand it to a Player with a share of the XP, change it and let it go', async () => {
    const list = [companion(1, 'Fang', { controllerId: player.id, sharesXp: true, hp: 4, notes: 'Bites strangers.' }), companion(2, 'Bors', { kind: 'hireling', monsterSlug: 'goblin' })]
    const { calls, routes } = backend('dm', list)
    const { wrapper } = await mountApp(`/campaigns/${ID}`, routes)
    const section = wrapper.get('[data-testid="companions"]')
    expect(section.get('[data-testid="companion-Fang"]').text()).toContain('wolf')
    expect(section.get('[data-testid="companion-Fang"]').text()).toContain('Run by Aria')
    expect(section.get('[data-testid="companion-Fang"]').text()).toContain('takes a share of the XP')
    expect(section.get('[data-testid="companion-Fang"]').text()).toContain('4 hit points')
    expect(section.get('[data-testid="companion-Fang"]').text()).toContain('Bites strangers.')
    expect(section.get('[data-testid="companion-Bors"]').text()).toContain('Hireling')
    expect(section.get('[data-testid="companion-Bors"]').text()).toContain('Run by the DM')
    expect(section.get('[data-testid="companion-Bors"]').text()).not.toContain('share')
    await expectAccessible(wrapper.element as Element)

    // A new one: a name and a creature are needed.
    const form = section.get('[data-testid="companion-form"]')
    expect((form.get('[data-testid="companion-save"]').element as HTMLButtonElement).disabled).toBe(true)
    await form.get('[data-testid="companion-name"]').setValue(' Rex ')
    await form.get('[data-testid="companion-creature"]').setValue(' wolf ')
    await form.get('[data-testid="companion-kind"]').setValue('hireling')
    await form.get('[data-testid="companion-controller"]').setValue(player.id)
    await form.get('[data-testid="companion-shares"]').setValue(true)
    await form.trigger('submit')
    await flushPromises()
    expect(JSON.parse((calls[0] ?? '').replace('POST ', ''))).toEqual({ name: 'Rex', kind: 'hireling', monsterSlug: 'wolf', controllerId: player.id, sharesXp: true, notes: '' })
    expect((form.get('[data-testid="companion-name"]').element as HTMLInputElement).value).toBe('')

    // Changing one loads it into the form; the DM takes Fang over.
    await section.get('[data-testid="companion-edit-Fang"]').trigger('click')
    expect((form.get('[data-testid="companion-name"]').element as HTMLInputElement).value).toBe('Fang')
    expect((form.get('[data-testid="companion-controller"]').element as HTMLSelectElement).value).toBe(player.id)
    await form.get('[data-testid="companion-controller"]').setValue('')
    await form.trigger('submit')
    await flushPromises()
    expect(calls[1]).toBe('PUT d1 {"name":"Fang","kind":"companion","monsterSlug":"wolf","sharesXp":true,"notes":"Bites strangers."}')
    await section.get('[data-testid="companion-edit-Bors"]').trigger('click')
    await form.get('[data-testid="companion-cancel"]').trigger('click')
    expect((form.get('[data-testid="companion-name"]').element as HTMLInputElement).value).toBe('')
    await section.get('[data-testid="companion-delete-Bors"]').trigger('click')
    await flushPromises()
    expect(calls[2]).toBe('DELETE d2')
  })

  it('says why an ally could not be kept', async () => {
    const { routes } = backend('dm', [companion(1, 'Fang')], 422)
    const { wrapper } = await mountApp(`/campaigns/${ID}`, routes)
    const form = wrapper.get('[data-testid="companion-form"]')
    await form.get('[data-testid="companion-name"]').setValue('Rex')
    await form.get('[data-testid="companion-creature"]').setValue('dragon')
    await form.trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="companion-problem"]').text()).toBe('there is no such creature in this Campaign')
    await wrapper.get('[data-testid="companion-delete-Fang"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="companion-problem"]').text()).toBe('That could not be saved. Try again shortly.')
  })

  it('shows a Player the party\'s allies, and nothing to change', async () => {
    const { routes } = backend('player', [companion(1, 'Fang', { controllerId: player.id })])
    const { wrapper } = await mountApp(`/campaigns/${ID}`, routes)
    expect(wrapper.get('[data-testid="companion-Fang"]').text()).toContain('Run by you')
    expect(wrapper.find('[data-testid="companion-form"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="companion-edit-Fang"]').exists()).toBe(false)
    // With no allies there is nothing for a Player to see.
    unmountAll()
    const none = await mountApp(`/campaigns/${ID}`, backend('player', []).routes)
    expect(none.wrapper.find('[data-testid="companions"]').exists()).toBe(false)
  })
})
