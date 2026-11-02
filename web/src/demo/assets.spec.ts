import { afterEach, describe, expect, it } from 'vitest'
import { demoSource, guardPictures, standIn } from './assets'

let undo: (() => void) | undefined
afterEach(() => {
  undo?.()
  undo = undefined
})

describe('pictures at an API path', () => {
  it('are drawn from the stand-in, so nothing asks the host for them', () => {
    undo = guardPictures()
    const img = document.createElement('img')
    img.src = '/api/v1/campaigns/c/maps/m/image'
    expect(img.getAttribute('src')).toBe(standIn)
    const image = document.createElementNS('http://www.w3.org/2000/svg', 'image')
    image.setAttribute('href', '/api/v1/campaigns/c/maps/m/image')
    expect(image.getAttribute('href')).toBe(standIn)
  })

  it('leave any other address and attribute alone', () => {
    undo = guardPictures()
    const img = document.createElement('img')
    img.src = 'data:image/png;base64,AA=='
    img.setAttribute('alt', '/api/v1/not-a-picture')
    expect(img.getAttribute('src')).toBe('data:image/png;base64,AA==')
    expect(img.getAttribute('alt')).toBe('/api/v1/not-a-picture')
    expect(demoSource('/favicon.svg')).toBe('/favicon.svg')
  })

  it('load as before once the guard is lifted', () => {
    guardPictures()()
    const img = document.createElement('img')
    img.setAttribute('src', '/api/v1/x')
    expect(img.getAttribute('src')).toBe('/api/v1/x')
  })
})
