import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'
import { crypt } from './maps'

/** The colour of one pixel of the world map's picture, as this screen is sent it. */
async function pixel(page: Page, x: number, y: number): Promise<number[]> {
  const src = String(await page.getByTestId('map-image').getAttribute('href'))
  return page.evaluate(async (at) => {
    const img = new Image()
    img.src = at.src
    await img.decode()
    const canvas = document.createElement('canvas')
    canvas.width = img.naturalWidth
    canvas.height = img.naturalHeight
    const ctx = canvas.getContext('2d')
    ctx?.drawImage(img, 0, 0)
    return Array.from(ctx?.getImageData(at.x, at.y, 1, 1).data ?? []).slice(0, 3)
  }, { src, x, y })
}

test('the world map is dark for a party without it and dimmed for one with it, and a secret place stays the DM\'s', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Found ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())

  await page.getByTestId('maps-link').click()
  await page.getByTestId('map-name').fill('Crypt')
  await page.getByTestId('map-file').setInputFiles({ name: 'crypt.png', mimeType: 'image/png', buffer: crypt })
  await page.getByRole('button', { name: 'Upload map' }).click()
  await expect(page.getByRole('heading', { name: 'Crypt' })).toBeVisible()
  await page.getByRole('link', { name: '← Maps' }).click()
  await page.getByTestId('use-default-world').click()
  await expect(page.getByRole('heading', { name: 'Default World' })).toBeVisible()
  await page.getByRole('link', { name: '← Maps' }).click()
  await page.getByRole('link', { name: '← Campaign' }).click()

  const context = await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-found-${stamp}` } })
  const player = await context.newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await page.getByTestId('scope-world').check()
  await page.getByTestId('world-choice').selectOption({ label: 'Default World' })
  await page.getByTestId('use-world').click()
  const place = async (name: string, hex: string) => {
    await page.getByTestId('world-node-name').fill(name)
    await page.locator(`[data-hex="${hex}"]`).click()
    await expect(page.locator(`[data-node="${name}"]`)).toHaveCount(1)
  }
  await place('Harbour', '8,6')
  await page.getByTestId('world-node-secret').check()
  await place('Hideout', '9,6')
  await page.getByTestId('world-node-secret').uncheck()
  await page.getByTestId('world-node-map').selectOption({ label: 'Crypt' })
  await place('Old Keep', '6,14')
  await page.getByTestId('world-node-map').selectOption('')
  await place('Far Tower', '14,10')
  await expect(page.locator('[data-node="Hideout"]')).toHaveText('Hideout (secret)')
  await expect(page.locator('[data-local-map="Old Keep"]')).toHaveCount(1)
  await page.getByTestId('world-tool-party').check()
  await page.locator('[data-hex="8,6"]').click()
  await expect(page.getByTestId('party-at')).toHaveText('The party is at Harbour.')

  // Without the world map: dark but for where the party stands. The Hideout is a hex away, and not there.
  await player.goto(new URL(page.url()).pathname)
  await expect(player.getByTestId('connection')).toHaveText('Live')
  await player.getByTestId('scope-world').check()
  await expect(player.getByTestId('party-marker')).toHaveCount(1)
  await expect(player.getByTestId('world-fog')).toHaveText('The party has no map of these lands: only where it has been shows.')
  await expect(player.locator('[data-hex="9,6"]')).toHaveClass(/cell--lit/)
  await expect(player.locator('[data-hex="14,10"]')).toHaveClass(/cell--unseen/)
  await expect(player.locator('[data-node]')).toHaveText(['Harbour'])
  const far = { x: 1000, y: 640 }
  expect(await pixel(player, far.x, far.y)).toEqual([0, 0, 0])
  const land = await pixel(page, far.x, far.y)
  expect(land).not.toEqual([0, 0, 0])

  // A local map the party finds shows at its place, and lights it.
  await page.getByTestId('found-maps').getByRole('button', { name: 'The party found it' }).nth(1).click()
  await expect(player.locator('[data-found-map="Old Keep"]')).toHaveCount(1)
  await expect(player.locator('[data-hex="6,14"]')).toHaveClass(/cell--lit/)
  await expect(player.locator('[data-node]')).toHaveText(['Harbour', 'Old Keep'])

  // With the world map: every place but the secret one, the land dimmed where the party has not been,
  // and the whole picture under it.
  await page.getByTestId('found-maps').getByRole('button', { name: 'The party found it' }).first().click()
  await expect(player.getByTestId('world-fog')).toHaveText('The party has this map: it is dimmed where the party has not been.')
  await expect(player.locator('[data-hex="14,10"]')).toHaveClass(/cell--remembered/)
  await expect(player.locator('[data-hex="9,6"]')).toHaveClass(/cell--lit/)
  await expect(player.locator('[data-node]')).toHaveText(['Harbour', 'Old Keep', 'Far Tower'])
  await expect(player.locator('[data-node="Hideout"]')).toHaveCount(0)
  await expect.poll(() => pixel(player, far.x, far.y)).toEqual(land)
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])

  // The DM sees everything throughout.
  await expect(page.locator('[data-node]')).toHaveCount(4)
  await expect(page.locator('[data-secret="Hideout"]')).toHaveCount(1)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
