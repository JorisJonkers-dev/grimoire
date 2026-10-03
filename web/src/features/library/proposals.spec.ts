import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Proposal, ProposalDetail } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { compare } from './fields'

const CAMP = '0190c7a8-0000-7000-8000-000000000001'
const PROP = '0190c7a8-0000-7000-8000-0000000000d1'
const HAG = '0190c7a8-0000-7000-8000-0000000000e1'
const at = '2026-10-01T20:00:00Z'
const lance: Proposal = {
  id: PROP, kind: 'spell', name: 'Frost Lance', fields: [{ name: 'Damage', value: '4d10' }, { name: 'Range', value: '60 ft' }],
  note: 'For my wizard', authorName: 'Tamsin', status: 'pending', message: '', createdAt: at, updatedAt: at,
}
const detail = (extra: Partial<ProposalDetail> = {}): ProposalDetail => ({
  proposal: lance, steps: [{ no: 1, action: 'submitted', message: 'For my wizard', by: 'Tamsin', createdAt: at }], ...extra,
})
const me = (myRole: 'dm' | 'player') => ({ id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: myRole, joinedAt: at, isMe: true })
const campaign = (myRole: 'dm' | 'player') => ({ id: CAMP, name: 'Morvain', ruleset: 'srd-2024', myRole, memberCount: 2, createdAt: at, me: me(myRole), members: [me(myRole)] })
type Sent = { method: string; path: string; body?: unknown }
const record = async (sent: Sent[], u: URL, req: Request) => {
  sent.push({ method: req.method, path: u.pathname, body: req.method === 'GET' ? undefined : await req.clone().json() })
}

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('proposal diff', () => {
  it('lines up both sides by field name and marks what changed', () => {
    expect(compare([{ name: 'HP', value: '52' }, { name: 'AC', value: '17' }], [{ name: 'HP', value: '40' }, { name: 'AC', value: '17' }, { name: 'Lair', value: 'Bog' }])).toEqual([
      { name: 'AC', now: '17', proposed: '17', changed: false },
      { name: 'HP', now: '52', proposed: '40', changed: true },
      { name: 'Lair', now: undefined, proposed: 'Bog', changed: true },
    ])
  })
})

describe('proposals page', () => {
  it('lists Proposals and sends a new one to the DM', async () => {
    const sent: Sent[] = []
    const { wrapper, router } = await mountApp(`/campaigns/${CAMP}/proposals`, {
      [`/api/v1/campaigns/${CAMP}/proposals/${PROP}`]: () => detail(),
      [`/api/v1/campaigns/${CAMP}/proposals`]: async (u, req) => {
        await record(sent, u, req)
        return req.method === 'POST' ? lance : [lance]
      },
      [`/api/v1/campaigns/${CAMP}`]: () => campaign('player'),
    })
    expect(wrapper.get('[data-testid="status-Frost Lance"]').text()).toBe('Waiting for the DM')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="proposal-kind"]').setValue('spell')
    await wrapper.get('[data-testid="proposal-name"]').setValue('Frost Lance')
    await wrapper.get('[data-testid="field-add"]').trigger('click')
    await wrapper.get('[data-testid="field-name-0"]').setValue('Damage')
    await wrapper.get('[data-testid="field-value-0"]').setValue('4d10')
    await wrapper.get('[data-testid="proposal-note"]').setValue('For my wizard')
    await wrapper.get('[data-testid="proposal-create"]').trigger('submit')
    await flushPromises()
    expect(sent.filter((s) => s.method === 'POST')).toEqual([
      { method: 'POST', path: `/api/v1/campaigns/${CAMP}/proposals`, body: { kind: 'spell', name: 'Frost Lance', fields: [{ name: 'Damage', value: '4d10' }], note: 'For my wizard' } },
    ])
    await vi.waitFor(() => {
      expect(router.currentRoute.value.name).toBe('proposal')
    })
  })

  it('says when there are none, and when one is refused or cannot be read', async () => {
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/proposals`, {
      [`/api/v1/campaigns/${CAMP}/proposals`]: (_u, req) =>
        req.method === 'POST' ? jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'give it a name of up to 80 characters' }, 422) : [],
    })
    expect(wrapper.get('[data-testid="proposals-empty"]').text()).toBe('No Proposals yet.')
    await wrapper.get('[data-testid="proposal-name"]').setValue('X')
    await wrapper.get('[data-testid="proposal-create"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="proposal-create"] [role="alert"]').text()).toBe('give it a name of up to 80 characters')
    const broken = await mountApp(`/campaigns/${CAMP}/proposals`, {
      [`/api/v1/campaigns/${CAMP}/proposals`]: () => jsonResponse({ type: 'about:blank', title: 'Forbidden', status: 403 }, 403),
    })
    expect(broken.wrapper.get('[data-testid="proposals-error"]').text()).toContain('could not be opened')
  })
})

describe('proposal page', () => {
  const current = {
    entry: { id: HAG, kind: 'spell' as const, name: 'Frost Lance', revision: 1, createdAt: at, updatedAt: at, fields: [] },
    baseName: 'Frost Lance', base: [], override: [], direct: true, via: [], fields: [{ name: 'Damage', value: '3d10' }, { name: 'Range', value: '60 ft' }],
  }

  it('shows the DM both sides and approves, asks for changes or declines', async () => {
    const sent: Sent[] = []
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/proposals/${PROP}`, {
      [`/api/v1/campaigns/${CAMP}/proposals/${PROP}`]: async (u, req) => {
        await record(sent, u, req)
        return detail({ current })
      },
      [`/api/v1/campaigns/${CAMP}`]: () => campaign('dm'),
    })
    expect(wrapper.get('[data-testid="proposal-diff"] h2').text()).toBe('Now')
    expect(wrapper.get('[data-testid="now-Damage"]').classes()).toContain('changed')
    expect(wrapper.get('[data-testid="proposed-Range"]').classes()).not.toContain('changed')
    expect(wrapper.get('[data-testid="proposal-note-shown"]').text()).toBe('Tamsin: For my wizard')
    expect(wrapper.get('[data-testid="request-changes"]').attributes('disabled')).toBeDefined()
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="field-value-0"]').setValue('3d8')
    await wrapper.get('[data-testid="review-message"]').setValue('Welcome')
    await wrapper.get('[data-testid="proposal-review"]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid="review-message"]').setValue('Lower it')
    await wrapper.get('[data-testid="request-changes"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="decline"]').trigger('click')
    await flushPromises()
    expect(sent.filter((s) => s.method === 'POST')).toEqual([
      {
        method: 'POST', path: `/api/v1/campaigns/${CAMP}/proposals/${PROP}/review`,
        body: { action: 'approve', message: 'Welcome', name: 'Frost Lance', fields: [{ name: 'Damage', value: '3d8' }, { name: 'Range', value: '60 ft' }] },
      },
      { method: 'POST', path: `/api/v1/campaigns/${CAMP}/proposals/${PROP}/review`, body: { action: 'request_changes', message: 'Lower it' } },
      { method: 'POST', path: `/api/v1/campaigns/${CAMP}/proposals/${PROP}/review`, body: { action: 'decline', message: '' } },
    ])
  })

  it('lets the author change and resend what the DM sent back', async () => {
    const sent: Sent[] = []
    const back = detail({
      proposal: { ...lance, status: 'changes_requested', message: 'Lower the damage' },
      steps: [{ no: 1, action: 'submitted', message: '', by: 'Tamsin', createdAt: at }, { no: 2, action: 'changes_requested', message: 'Lower the damage', by: 'Joris', createdAt: at }],
    })
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/proposals/${PROP}`, {
      [`/api/v1/campaigns/${CAMP}/proposals/${PROP}`]: async (u, req) => {
        await record(sent, u, req)
        return req.method === 'PUT' ? jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'only a Proposal the DM asked changes to can be sent again' }, 422) : back
      },
      [`/api/v1/campaigns/${CAMP}`]: () => campaign('player'),
    })
    expect(wrapper.get('[data-testid="proposal-diff"] h2').text()).toBe('A new entry')
    expect(wrapper.get('[data-testid="proposal-message"]').text()).toBe('The DM: Lower the damage')
    expect(wrapper.get('[data-testid="proposal-steps"]').text()).toContain('Changes asked for by Joris')
    expect(wrapper.find('[data-testid="proposal-review"]').exists()).toBe(false)
    await wrapper.get('[data-testid="field-value-0"]').setValue('3d10')
    await wrapper.get('[data-testid="resubmit-note"]').setValue('Lowered')
    await wrapper.get('[data-testid="proposal-resubmit"]').trigger('submit')
    await flushPromises()
    expect(sent.filter((s) => s.method === 'PUT')).toEqual([
      { method: 'PUT', path: `/api/v1/campaigns/${CAMP}/proposals/${PROP}`, body: { name: 'Frost Lance', fields: [{ name: 'Damage', value: '3d10' }, { name: 'Range', value: '60 ft' }], note: 'Lowered' } },
    ])
    expect(wrapper.get('[data-testid="proposal-problem"]').text()).toBe('only a Proposal the DM asked changes to can be sent again')
  })

  it('says when the Proposal is not there for the caller', async () => {
    const { wrapper } = await mountApp(`/campaigns/${CAMP}/proposals/${PROP}`, { [`/api/v1/campaigns/${CAMP}`]: () => campaign('player') })
    expect(wrapper.get('[data-testid="proposal-error"]').text()).toContain('no such Proposal')
  })
})
