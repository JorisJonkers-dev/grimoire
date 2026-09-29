import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import GalleryPage from './GalleryPage.vue'

describe('GalleryPage', () => {
  it('renders every component family accessibly', async () => {
    const w = mount(GalleryPage, { attachTo: document.body })
    for (const heading of ['Buttons', 'Hotbar', 'Tokens', 'Conditions and odds', 'Dice']) {
      expect(w.text()).toContain(heading)
    }
    await expectAccessible(w.element as Element)
    w.unmount()
  })
})
