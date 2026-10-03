import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { DiceSet } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const me = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'a@example.com',
  admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false,
}
const id = (n: number) => `0190c7a8-0000-7000-8000-0000000000${String(40 + n)}`
const set = (n: number, name: string, extra: Partial<DiceSet> = {}): DiceSet => ({
  id: id(n), name, hasImage: false, sharing: 'private', review: 'none', mine: true, copy: false, by: 'aria', updatedAt: '2026-10-03T10:00:00Z',
  design: { dice: { d20: { pattern: 'marble', body: '#102030', numbers: '#ffffff' } } }, ...extra,
})
const picture = '/api/v1/dice-sets/x/image?v=abc'

afterEach(() => { unmountAll() })

/** A small back end: what the page asked for, and the sets it answers with. */
function backend(state: { mine: DiceSet[]; chosen?: string; shared?: DiceSet[] }, fail: Record<string, number> = {}) {
  const calls: string[] = []
  const said = async (req: Request) => (req.method === 'GET' || req.method === 'DELETE' ? '' : req.headers.get('Content-Type') === 'application/octet-stream' ? `${String((await req.arrayBuffer()).byteLength)} bytes` : await req.text())
  const routes = {
    '/api/v1/dice-sets/shared': () => ({ items: state.shared ?? [] }),
    '/api/v1/dice-sets/chosen': async (_u: URL, req: Request) => {
      calls.push(`choose ${await said(req)}`)
      return new Response(null, { status: fail.choose ?? 204 })
    },
    '/api/v1/dice-sets/': async (url: URL, req: Request) => {
      const [setId = '', verb = 'set'] = url.pathname.replace('/api/v1/dice-sets/', '').split('/')
      const what = `${req.method} ${verb}`
      calls.push(`${what} ${setId.slice(-2)} ${await said(req)}`.trim())
      if (fail[what]) return jsonResponse({ status: fail[what], title: 'No' }, fail[what])
      const found = state.mine.find((s) => s.id === setId) ?? state.shared?.find((s) => s.id === setId)
      if (what === 'DELETE set') return new Response(null, { status: 204 })
      if (what === 'PUT image' && found) Object.assign(found, { hasImage: true, imageUrl: picture })
      if (what === 'DELETE image' && found) Object.assign(found, { hasImage: false, imageUrl: undefined })
      return jsonResponse(found, what === 'POST copy' ? 201 : 200)
    },
    '/api/v1/dice-sets': async (_u: URL, req: Request) => {
      if (req.method === 'GET') return { items: state.mine, ...(state.chosen ? { chosen: state.chosen } : {}) }
      const body = (await req.clone().json()) as { name: string; design: DiceSet['design'] }
      calls.push(`create ${await said(req)}`)
      if (fail.create) return jsonResponse({ status: fail.create, title: 'No' }, fail.create)
      const made = set(9, body.name, { design: body.design })
      state.mine.push(made)
      return jsonResponse(made, 201)
    },
    '/api/v1/account': () => me,
  }
  return { calls, routes }
}

describe('the Dice Sets page', () => {
  it('lists your sets and copies, and chooses the one you roll with', async () => {
    const state = { mine: [set(1, 'Ember'), set(2, 'Frost', { copy: true, by: 'bram', review: 'approved', hasImage: true, imageUrl: picture })], chosen: id(1) }
    const { calls, routes } = backend(state)
    const { wrapper } = await mountApp('/dice-sets', routes)
    const checked = (name: string) => (wrapper.get(`[data-testid="dice-choice-${name}"]`).element as HTMLInputElement).checked
    expect(wrapper.get('[data-testid="dice-sets-link"]').attributes('href')).toBe('/dice-sets')
    expect([checked('plain'), checked('Ember'), checked('Frost')]).toEqual([false, true, false])
    // A copy says whose design it is, and can be rolled with or thrown away: not edited, not shared on.
    expect(wrapper.get('[data-testid="copy-of-Frost"]').text()).toBe('a copy, by bram')
    expect(wrapper.find('[data-testid="edit-Frost"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="sharing-Frost"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="review-Frost"]').exists()).toBe(false)
    // Its picture shows only on the dice it was placed on.
    expect(wrapper.find('[data-testid="dice-set-Frost"] [data-testid="sheet-picture"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="dice-set-Ember"] [data-testid="dice-sheet"]').classes()).toContain('sheet--marble')
    expect(wrapper.get('[data-testid="shared-none"]').text()).toContain('Nothing yet')
    await expectAccessible(wrapper.element as Element)

    await wrapper.get('[data-testid="dice-choice-Frost"]').setValue(true)
    await flushPromises()
    await wrapper.get('[data-testid="dice-choice-plain"]').setValue(true)
    await flushPromises()
    await wrapper.get('[data-testid="delete-Frost"]').trigger('click')
    await flushPromises()
    expect(calls).toEqual([`choose {"diceSetId":"${id(2)}"}`, 'choose {}', 'DELETE set 42'])
    expect(wrapper.find('[data-testid="dice-sets-problem"]').exists()).toBe(false)
  })

  it('shares a set, and says where one with a picture stands with the Admins', async () => {
    const state = { mine: [set(1, 'Ember', { sharing: 'everyone', review: 'pending', hasImage: true, imageUrl: picture }), set(2, 'Frost')] }
    const { calls, routes } = backend(state)
    const { wrapper } = await mountApp('/dice-sets', routes)
    expect((wrapper.get('[data-testid="sharing-Ember"]').element as HTMLSelectElement).value).toBe('everyone')
    expect(wrapper.get('[data-testid="review-Ember"]').text()).toContain('Waiting for an Admin')
    expect(wrapper.find('[data-testid="review-Frost"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="sharing-Frost"] option').map((o) => o.text())).toEqual(['Only you', 'Your Friends', 'Everyone'])
    await wrapper.get('[data-testid="sharing-Frost"]').setValue('friends')
    await flushPromises()
    expect(calls).toEqual(['PUT sharing 42 {"sharing":"friends"}'])
  })

  it('designs a set die by die and places its picture on the unwrapped faces', async () => {
    const state = { mine: [] as DiceSet[] }
    const { calls, routes } = backend(state)
    const { wrapper } = await mountApp('/dice-sets', routes)
    expect(wrapper.find('[data-testid="dice-set-editor"]').exists()).toBe(false)
    await wrapper.get('[data-testid="dice-set-new"]').trigger('click')
    expect(wrapper.find('[data-testid="dice-set-new"]').exists()).toBe(false)
    const editor = () => wrapper.get('[data-testid="dice-set-editor"]')
    expect((editor().get('[data-testid="set-save"]').element as HTMLButtonElement).disabled).toBe(true)
    // The d20 is in hand first; its sheet has twenty faces, the d6's has six.
    expect(editor().get('[data-testid="die-d20"]').attributes('aria-pressed')).toBe('true')
    expect(editor().findAll('[data-testid="dice-sheet"] text').map((t) => t.text())).toEqual(Array.from({ length: 20 }, (_, i) => String(i + 1)))
    await editor().get('[data-testid="look-pattern"]').setValue('stripes')
    await editor().get('[data-testid="look-body"]').setValue('#102030')
    await editor().get('[data-testid="look-numbers"]').setValue('#ffffff')
    expect(editor().get('[data-testid="dice-sheet"]').classes()).toContain('sheet--stripes')
    expect(editor().get('[data-testid="dice-sheet"]').attributes('style')).toContain('--body: #102030')
    await editor().get('[data-testid="die-d6"]').trigger('click')
    expect(editor().findAll('[data-testid="dice-sheet"] text')).toHaveLength(6)
    expect(editor().get('[data-testid="dice-sheet"]').classes()).toContain('sheet--plain')
    await editor().get('[data-testid="look-pattern"]').setValue('speckled')
    // A new set has to exist before it can carry a picture.
    expect(editor().get('[data-testid="picture-later"]').text()).toContain('Save the set first')
    expect(editor().find('[data-testid="set-picture-file"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)

    await editor().get('[data-testid="set-name"]').setValue(' Storm ')
    await editor().trigger('submit')
    await flushPromises()
    const created = JSON.parse((calls[0] ?? '').replace('create ', '')) as { name: string; design: DiceSet['design'] }
    expect(created.name).toBe('Storm')
    expect(created.design.dice.d20).toEqual({ pattern: 'stripes', body: '#102030', numbers: '#ffffff' })
    expect(created.design.dice.d6?.pattern).toBe('speckled')
    expect(created.design.dice.d4).toEqual({ pattern: 'plain', body: '#7a1f1a', numbers: '#f3d27a' })

    // Saved, the editor stays on the set, back on its d20; now it takes a picture.
    expect(editor().get('h2').text()).toBe('Edit Storm')
    expect(editor().get('[data-testid="dice-sheet"]').classes()).toContain('sheet--stripes')
    const file = editor().get('[data-testid="set-picture-file"]')
    Object.defineProperty(file.element, 'files', { value: [new File([new Uint8Array(8)], 'p.png', { type: 'image/png' })], configurable: true })
    await file.trigger('change')
    await flushPromises()
    expect(calls[1]).toBe('PUT image 49 8 bytes')
    // The picture goes on a die only where it is put; then it is moved, sized and turned on the sheet.
    expect(editor().find('[data-testid="sheet-picture"]').exists()).toBe(false)
    expect(editor().find('[data-testid="picture-x"]').exists()).toBe(false)
    await editor().get('[data-testid="picture-on"]').setValue(true)
    expect(editor().get('[data-testid="sheet-picture"]').attributes('style')).toContain('left: 50%; top: 50%; width: 100%')
    await editor().get('[data-testid="picture-x"]').setValue('0.25')
    await editor().get('[data-testid="picture-y"]').setValue('0.75')
    await editor().get('[data-testid="picture-scale"]').setValue('2')
    await editor().get('[data-testid="picture-rotation"]').setValue('90')
    expect(editor().get('[data-testid="sheet-picture"]').attributes('style')).toContain('left: 25%; top: 75%; width: 200%; transform: translate(-50%, -50%) rotate(90deg)')
    expect(editor().get('[data-testid="sheet-picture"]').attributes('src')).toBe(picture)
    await expectAccessible(wrapper.element as Element)
    // One look for every die, picture and all; each die keeps a placement of its own afterwards.
    await editor().get('[data-testid="look-all"]').trigger('click')
    await editor().get('[data-testid="die-d4"]').trigger('click')
    await editor().get('[data-testid="picture-scale"]').setValue('3')
    await editor().get('[data-testid="die-d8"]').trigger('click')
    await editor().get('[data-testid="picture-on"]').setValue(false)
    await editor().trigger('submit')
    await flushPromises()
    const edited = JSON.parse((calls[2] ?? '').replace('PUT set 49 ', '')) as { design: DiceSet['design'] }
    const placed = { x: 0.25, y: 0.75, scale: 2, rotation: 90 }
    expect(edited.design.dice.d20).toEqual({ pattern: 'stripes', body: '#102030', numbers: '#ffffff', image: placed })
    expect(edited.design.dice.d4).toEqual({ pattern: 'stripes', body: '#102030', numbers: '#ffffff', image: { ...placed, scale: 3 } })
    expect(edited.design.dice.d8).toEqual({ pattern: 'stripes', body: '#102030', numbers: '#ffffff' })
    expect(edited.design.dice.d6?.image).toEqual(placed)

    await editor().get('[data-testid="set-picture-remove"]').trigger('click')
    await flushPromises()
    expect(calls[3]).toBe('DELETE image 49')
    expect(editor().find('[data-testid="picture-on"]').exists()).toBe(false)
    await editor().get('[data-testid="set-done"]').trigger('click')
    expect(wrapper.find('[data-testid="dice-set-editor"]').exists()).toBe(false)
    // An existing set opens in the editor with its own looks, and closes when it is deleted.
    await wrapper.get('[data-testid="edit-Storm"]').trigger('click')
    expect((editor().get('[data-testid="set-name"]').element as HTMLInputElement).value).toBe('Storm')
    await wrapper.get('[data-testid="delete-Storm"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="dice-set-editor"]').exists()).toBe(false)
  })

  it('takes a copy of a set shared with you', async () => {
    const state = { mine: [] as DiceSet[], shared: [set(3, 'Gale', { mine: false, by: 'bram', sharing: 'friends' })] }
    const { calls, routes } = backend(state)
    const { wrapper } = await mountApp('/dice-sets', routes)
    expect(wrapper.get('[data-testid="shared-set-Gale"]').text()).toContain('by bram')
    await wrapper.get('[data-testid="copy-Gale"]').trigger('click')
    await flushPromises()
    expect(calls).toEqual(['POST copy 43'])
    expect(wrapper.find('[data-testid="dice-sets-problem"]').exists()).toBe(false)
  })

  it('says so when something could not be done', async () => {
    const state = { mine: [set(1, 'Ember', { hasImage: true, imageUrl: picture })], shared: [set(3, 'Gale', { mine: false, by: 'bram' })] }
    const { routes } = backend(state, { 'POST copy': 409, choose: 500, 'PUT set': 500, 'PUT image': 500, 'DELETE image': 500, create: 500, 'PUT sharing': 500, 'DELETE set': 500 })
    const { wrapper } = await mountApp('/dice-sets', routes)
    const problem = () => wrapper.get('[data-testid="dice-sets-problem"]').text()
    await wrapper.get('[data-testid="copy-Gale"]').trigger('click')
    await flushPromises()
    expect(problem()).toBe('You already have a copy of that set.')
    await wrapper.get('[data-testid="dice-choice-Ember"]').setValue(true)
    await flushPromises()
    expect(problem()).toBe('The set could not be chosen. Try again shortly.')
    await wrapper.get('[data-testid="sharing-Ember"]').setValue('everyone')
    await flushPromises()
    expect(problem()).toBe('The set could not be shared. Try again shortly.')

    await wrapper.get('[data-testid="edit-Ember"]').trigger('click')
    const editor = wrapper.get('[data-testid="dice-set-editor"]')
    const said = () => editor.get('[data-testid="set-problem"]').text()
    await editor.trigger('submit')
    await flushPromises()
    expect(said()).toBe('The Dice Set could not be saved. Try again shortly.')
    const file = editor.get('[data-testid="set-picture-file"]')
    const pick = async (f?: File) => {
      Object.defineProperty(file.element, 'files', { value: f ? [f] : [], configurable: true })
      await file.trigger('change')
      await flushPromises()
    }
    await pick(new File(['x'], 'notes.txt', { type: 'text/plain' }))
    expect(said()).toBe('Pictures must be PNG, JPEG or WebP.')
    await pick(new File([new Uint8Array(4)], 'p.png', { type: 'image/png' }))
    expect(said()).toBe('The picture could not be saved. Try again shortly.')
    await pick()
    expect(said()).toBe('The picture could not be saved. Try again shortly.')
    await editor.get('[data-testid="set-picture-remove"]').trigger('click')
    await flushPromises()
    expect(said()).toBe('The Dice Set could not be saved. Try again shortly.')
    await wrapper.get('[data-testid="delete-Ember"]').trigger('click')
    await flushPromises()
    expect(problem()).toBe('The set could not be deleted. Try again shortly.')

    unmountAll()
    const fresh = await mountApp('/dice-sets', routes)
    await fresh.wrapper.get('[data-testid="dice-set-new"]').trigger('click')
    await fresh.wrapper.get('[data-testid="set-name"]').setValue('Storm')
    await fresh.wrapper.get('[data-testid="dice-set-editor"]').trigger('submit')
    await flushPromises()
    expect(fresh.wrapper.get('[data-testid="set-problem"]').text()).toBe('The Dice Set could not be saved. Try again shortly.')
  })

  it('needs an Account, and says so when the sets cannot be read', async () => {
    const none = await mountApp('/dice-sets', { '/api/v1/dice-sets': () => jsonResponse({ status: 403, title: 'No Account' }, 403) })
    expect(none.wrapper.get('[data-testid="dice-sets-no-account"]').text()).toContain('need a Grimoire Account')
    expect(none.wrapper.find('[data-testid="dice-sets-link"]').exists()).toBe(false)
    unmountAll()
    const down = await mountApp('/dice-sets', { '/api/v1/dice-sets': () => jsonResponse({ status: 500, title: 'Down' }, 500), '/api/v1/account': () => me })
    expect(down.wrapper.get('[role="alert"]').text()).toBe('Your Dice Sets could not be read.')
  })
})

describe('the Dice Set review page', () => {
  it('shows an Admin each picture that waits, to approve or turn down', async () => {
    const calls: string[] = []
    const waiting = [set(1, 'Ember', { mine: false, sharing: 'everyone', review: 'pending', hasImage: true, imageUrl: picture, imageVersion: '0123456789ab' })]
    let fail = 0
    const { wrapper } = await mountApp('/admin/dice-sets', {
      '/api/v1/admin/dice-sets/': async (url, req) => {
        calls.push(`${url.pathname.split('/').at(-2)?.slice(-2) ?? ''} ${await req.text()}`)
        if (fail) return jsonResponse({ status: fail, title: 'No' }, fail)
        const [done] = waiting.splice(0, 1)
        return done
      },
      '/api/v1/admin/dice-sets': () => ({ items: waiting }),
      '/api/v1/account': () => ({ ...me, admin: true, adminPowers: true }),
    })
    const card = wrapper.get('[data-testid="review-set-Ember"]')
    expect(card.text()).toContain('by aria')
    expect(card.get('[data-testid="review-picture"]').attributes()).toMatchObject({ src: picture, alt: 'The picture on Ember' })
    await expectAccessible(wrapper.element as Element)
    fail = 500
    await card.get('[data-testid="review-reject"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="dice-review-problem"]').text()).toContain('not saved')
    // The owner swapped the picture meanwhile: the decision is refused and the Admin looks again.
    fail = 409
    const [first] = waiting
    if (first) Object.assign(first, { imageVersion: 'ba9876543210' })
    await wrapper.get('[data-testid="review-approve"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="dice-review-problem"]').text()).toContain('changed while you were looking')
    fail = 0
    await wrapper.get('[data-testid="review-approve"]').trigger('click')
    await flushPromises()
    // Each decision names the picture that was on screen when it was made.
    expect(calls).toEqual(['41 {"approve":false,"picture":"0123456789ab"}', '41 {"approve":true,"picture":"0123456789ab"}', '41 {"approve":true,"picture":"ba9876543210"}'])
    expect(wrapper.get('[data-testid="dice-review-none"]').text()).toBe('No pictures wait to be checked.')
  })

  it('is closed to anyone else, and linked from the Admin page', async () => {
    const { wrapper } = await mountApp('/admin/dice-sets', { '/api/v1/admin/dice-sets': () => jsonResponse({ status: 403, title: 'No' }, 403), '/api/v1/account': () => me })
    expect(wrapper.get('[data-testid="dice-review-forbidden"]').text()).toContain('Only an Admin')
    unmountAll()
    const admin = await mountApp('/admin', { '/api/v1/account': () => ({ ...me, admin: true, adminPowers: true }) })
    expect(admin.wrapper.get('[data-testid="admin-dice-link"]').attributes('href')).toBe('/admin/dice-sets')
  })
})
