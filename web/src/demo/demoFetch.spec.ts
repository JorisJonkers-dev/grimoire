import { describe, expect, it } from 'vitest'
import { createDemo } from './demo'
import { DemoProblem, forbidden, type Handlers } from './operations'

const base = 'https://demo.invalid'

async function ask(handlers: Handlers, path: string, init: RequestInit = {}) {
  const demo = createDemo('player', handlers)
  const response = await demo.fetch(new Request(`${base}${path}`, init))
  const text = await response.text()
  return { demo, status: response.status, body: text ? (JSON.parse(text) as unknown) : undefined }
}

describe('the demo fetch', () => {
  it('answers an operation from its handler, with the caller and the world', async () => {
    const { status, body, demo } = await ask({ getAccount: ({ who }) => who.account as never }, '/api/v1/account')
    expect(status).toBe(200)
    expect(body).toMatchObject({ username: 'ash' })
    expect(demo.answers).toEqual([{ what: 'getAccount GET /api/v1/account', status: 200 }])
  })

  it('takes a request given as a URL and options', async () => {
    const demo = createDemo('dm', { getAccount: ({ who }) => who.account as never })
    const response = await demo.fetch(`${base}/api/v1/account`, { method: 'GET' })
    expect(await response.json()).toMatchObject({ username: 'morgan' })
  })

  it('reads a query the way the contract types it', async () => {
    let seen: unknown
    const { status } = await ask({ listSpells: ({ query }) => { seen = query; return { items: [] } } }, '/api/v1/compendium/spells?level=3&q=fire')
    expect(status).toBe(200)
    expect(seen).toMatchObject({ level: 3, q: 'fire' })
  })

  it('keeps text that only looks like a number when the contract asks for text', async () => {
    let seen: unknown
    await ask({ listSpells: ({ query }) => { seen = query; return { items: [] } } }, '/api/v1/compendium/spells?q=3')
    expect(seen).toMatchObject({ q: '3' })
  })

  it('refuses a query parameter given twice where the contract takes one', async () => {
    const { status } = await ask({ search: () => ({ items: [] }) as never }, '/api/v1/search?q=bell&q=book')
    expect(status).toBe(400)
  })

  it('reads a JSON body', async () => {
    let seen: unknown
    const { status } = await ask(
      { updateAccount: ({ body, who }) => { seen = body; return who.account as never } },
      '/api/v1/account',
      { method: 'PUT', body: JSON.stringify({ username: 'ash', nickname: 'Ashen', email: 'ash@demo.invalid' }), headers: { 'Content-Type': 'application/json' } },
    )
    expect(status).toBe(200)
    expect(seen).toEqual({ username: 'ash', nickname: 'Ashen', email: 'ash@demo.invalid' })
  })

  it('refuses a request that breaks the contract, as the API does', async () => {
    const { status, demo } = await ask({ updateAccount: ({ who }) => who.account as never }, '/api/v1/account', { method: 'PUT', body: 'not json' })
    expect(status).toBe(400)
    expect(demo.answers).toEqual([{ what: 'updateAccount PUT /api/v1/account', status: 400 }])
  })

  it('answers no content when a handler returns nothing', async () => {
    const { status, body } = await ask({ signOut: () => undefined }, '/api/v1/sign-out', { method: 'POST' })
    expect(status).toBe(204)
    expect(body).toBeUndefined()
  })

  it('turns a refusal into the problem the API would send', async () => {
    const { status, body, demo } = await ask({ getAccount: () => { throw forbidden() } }, '/api/v1/account')
    expect(status).toBe(403)
    expect(body).toEqual({ type: 'about:blank', title: 'Forbidden', status: 403 })
    expect(demo.answers).toEqual([{ what: 'getAccount GET /api/v1/account', status: 403 }])
  })

  it('lets a handler bug surface instead of hiding it as a refusal', async () => {
    await expect(ask({ getAccount: () => { throw new Error('bug') } }, '/api/v1/account')).rejects.toThrow('bug')
  })

  it('says so when an operation has no handler', async () => {
    const { status, demo } = await ask({}, '/api/v1/account')
    expect(status).toBe(501)
    expect(demo.unhandled).toEqual(['getAccount GET /api/v1/account'])
  })

  it('answers a path the contract does not have with not found', async () => {
    const { status, demo } = await ask({}, '/api/v1/nowhere')
    expect(status).toBe(404)
    expect(demo.unhandled).toEqual(['GET /api/v1/nowhere'])
  })

  it('matches a literal segment before a path parameter, as the API router does', async () => {
    const { demo } = await ask({ listLibraryCollections: () => [], getLibraryEntry: () => { throw new DemoProblem(404, 'no') } }, '/api/v1/library/collections')
    expect(demo.answers.map((a) => a.what)).toEqual(['listLibraryCollections GET /api/v1/library/collections'])
  })

  it('reads a number in the path the way the contract types it', async () => {
    let seen: unknown
    const entry = '0190c7a8-0000-7000-8000-00000000f501'
    await ask({ restoreLibraryRevision: ({ path }) => { seen = path; throw forbidden() } }, `/api/v1/library/${entry}/revisions/2/restore`, { method: 'POST' })
    expect(seen).toEqual({ entryId: entry, revisionNo: 2 })
  })

  it('hands path parameters over decoded', async () => {
    let seen: unknown
    const id = '0190c7a8-0000-7000-8000-00000000c001'
    await ask({ getCampaign: ({ path }) => { seen = path; throw forbidden() } }, `/api/v1/campaigns/${id}`)
    expect(seen).toEqual({ campaignId: id })
  })
})
