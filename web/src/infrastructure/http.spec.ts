import { describe, expect, it } from 'vitest'
import { client } from './api/client.gen'
import { configureApi } from './http'

describe('configureApi', () => {
  it('defaults to the page origin', () => {
    configureApi({ baseUrl: 'http://elsewhere' })
    configureApi()
    expect(client.getConfig().baseUrl).toBe('')
  })
})
