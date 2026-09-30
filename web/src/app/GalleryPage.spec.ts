import { describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountWithQuery } from '@/test/mountWithQuery'
import GalleryPage from './GalleryPage.vue'

describe('GalleryPage', () => {
  it('renders every component family accessibly', async () => {
    const w = mountWithQuery(GalleryPage, () => Promise.reject(new Error('offline')))
    document.body.appendChild(w.element)
    for (const heading of ['Buttons', 'Hotbar', 'Tokens', 'Conditions and odds', 'Dice', 'Movement and sight']) {
      expect(w.text()).toContain(heading)
    }
    await expectAccessible(w.element as Element)
    w.unmount()
  })
})
