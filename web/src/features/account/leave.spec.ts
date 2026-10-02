import { afterEach, describe, expect, it, vi } from 'vitest'
import { leaveFor, returnTo } from './leave'

afterEach(() => { vi.restoreAllMocks() })

describe('leaving for the external login', () => {
  it('remembers where to come back to, once, and only inside Grimoire', () => {
    leaveFor('#signing-in', '/campaigns/1')
    expect(returnTo()).toBe('/campaigns/1')
    expect(returnTo()).toBe('/')
    leaveFor('#signing-in', '//evil.example')
    expect(returnTo()).toBe('/')
  })

  it('goes home when the browser keeps no storage', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied') })
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('denied') })
    leaveFor('#signing-in', '/campaigns')
    expect(returnTo()).toBe('/')
  })
})
