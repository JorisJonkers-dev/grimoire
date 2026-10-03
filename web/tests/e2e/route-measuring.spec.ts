import AxeBuilder from '@axe-core/playwright'
import { devices, expect, test } from '@playwright/test'

test('a player measures a route on the world map in hexes, miles and hours, zoomed in or not', async ({ page, browser }, info) => {
  test.skip(!['desktop', 'phone'].includes(info.project.name), 'a desk and a phone are the two ways to measure')
  const phone = info.project.name === 'phone'
  const stamp = String(Date.now())
  const dm = await (await browser.newContext({ ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 }, extraHTTPHeaders: { 'X-User-Id': `e2e-measure-dm-${stamp}` } })).newPage()
  await dm.goto('/campaigns')
  await dm.getByTestId('campaign-name').fill(`Measure ${stamp}`)
  await dm.getByTestId('campaign-display-name').fill('DM')
  await dm.getByRole('button', { name: 'Start as DM' }).click()
  await dm.getByTestId('create-invite').click()
  const link = new URL(await dm.getByTestId('invite-link').inputValue())

  // The Default World, at twelve miles to a hex.
  await dm.getByTestId('maps-link').click()
  await dm.getByTestId('use-default-world').click()
  await expect(dm.getByRole('heading', { name: 'Default World' })).toBeVisible()
  await dm.getByTestId('scale-miles').fill('12')
  await dm.getByRole('button', { name: 'Save calibration' }).click()
  await expect(dm.getByTestId('calibration-saved')).toBeVisible()
  await dm.getByRole('link', { name: '← Maps' }).click()
  await dm.getByRole('link', { name: '← Campaign' }).click()

  await page.goto(link.pathname + link.hash)
  await page.getByTestId('join-display-name').fill('Aria')
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await expect(page.getByTestId('member-list')).toContainText('Aria (you)')

  await dm.getByTestId('start-session').click()
  await expect(dm.getByTestId('connection')).toHaveText('Live')
  await dm.getByTestId('scope-world').check()
  await dm.getByTestId('world-choice').selectOption({ label: 'Default World' })
  await dm.getByTestId('use-world').click()
  await dm.getByTestId('world-node-name').fill('Harbour')
  await dm.locator('[data-hex="8,6"]').click()
  await expect(dm.locator('[data-node="Harbour"]')).toHaveCount(1)
  await dm.getByTestId('world-tool-party').check()
  await dm.locator('[data-hex="8,6"]').click()
  await expect(dm.getByTestId('party-at')).toHaveText('The party is at Harbour.')

  await page.goto(new URL(dm.url()).pathname)
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await page.getByTestId('scope-world').check()
  await expect(page.getByTestId('party-marker')).toHaveCount(1)
  const hit = (hex: string) => (phone ? page.locator(`[data-hex="${hex}"]`).tap() : page.locator(`[data-hex="${hex}"]`).click())

  await page.getByTestId('measure-toggle').click()
  await expect(page.getByTestId('measure-hint')).toHaveText('Tap the map to mark the route. Each tap adds a point.')
  await hit('8,6')
  await hit('10,6')
  await expect(page.getByTestId('measure-result')).toHaveText('2 hexes · 24 miles · 8 h at a normal pace')
  await expect(page.getByTestId('measure-line')).toHaveCount(1)
  await hit('10,7')
  await expect(page.getByTestId('measure-result')).toHaveText('3 hexes · 36 miles · 2 days (12 h on the road) at a normal pace')
  await page.getByTestId('measure-pace').selectOption('fast')
  await expect(page.getByTestId('measure-result')).toHaveText('3 hexes · 36 miles · 2 days (9 h on the road) at a fast pace')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  if (phone) {
    // Two fingers spreading on the map zoom it, and the route can be carried on from there.
    const zoomer = page.getByTestId('zoomer')
    await expect(zoomer).toHaveAttribute('style', /width: 100%/)
    const cdp = await page.context().newCDPSession(page)
    const stage = await page.getByTestId('stage').boundingBox()
    const mid = { x: (stage?.x ?? 0) + (stage?.width ?? 0) / 2, y: (stage?.y ?? 0) + 120 }
    const fingers = (spread: number) => [{ x: mid.x - spread, y: mid.y, id: 1 }, { x: mid.x + spread, y: mid.y, id: 2 }]
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: fingers(30) })
    for (const spread of [45, 60]) await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: fingers(spread) })
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
    await expect(zoomer).toHaveAttribute('style', /width: 200%/)
    await expect(page.locator('[data-waypoint]')).toHaveCount(3)
    await page.locator('[data-hex="9,7"]').scrollIntoViewIfNeeded()
    await hit('9,7')
    await expect(page.getByTestId('measure-result')).toHaveText('4 hexes · 48 miles · 2 days (12 h on the road) at a fast pace')
  }

  // The DM never saw the player's measure; taking points back measures what is left.
  await expect(dm.locator('[data-waypoint]')).toHaveCount(0)
  await page.getByTestId('measure-clear').click()
  await expect(page.locator('[data-waypoint]')).toHaveCount(0)
  await expect(page.getByTestId('measure-result')).toHaveCount(0)
})
