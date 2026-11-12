import { expect, test } from '@playwright/test'
import { demoPages } from '../../src/demo/pages'
import { violations } from './axe'

// The demo build (src/demo) seeds a record for every route and every caller that route serves, so
// one pass over it checks every page of the app with axe, including the ones no other spec opens.
// It is served on its own port, and needs no API.
test.use({ baseURL: process.env.E2E_DEMO_ORIGIN ?? 'http://localhost:4174' })

for (const { name, path, personas } of demoPages) {
  for (const persona of personas) {
    test(`${name} as ${persona} passes axe`, async ({ page }) => {
      await page.goto('/#/compendium/spells')
      await page.locator('label', { has: page.getByTestId(`demo-as-${persona}`) }).click()
      await page.evaluate((hash) => { location.hash = hash }, path)
      await expect(page.locator('h1').first()).toBeVisible()
      const found = await violations(page)
      expect(found.map((v) => `${v.id}: ${v.nodes.map((n) => n.html.slice(0, 160)).join(' | ')}`)).toEqual([])
    })
  }
}
