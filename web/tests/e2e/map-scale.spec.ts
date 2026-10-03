import AxeBuilder from '@axe-core/playwright'
import { expect, type Locator, type Page, test } from '@playwright/test'
import { crypt } from './maps'

/** Drags a calibration point to a place on the picture, in the picture's own pixels. */
async function dragTo(page: Page, point: Locator, x: number, y: number) {
  await point.scrollIntoViewIfNeeded()
  const from = await point.boundingBox()
  const board = await page.getByTestId('map-board').boundingBox()
  if (!from || !board) throw new Error('the board is not on the page')
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2)
  await page.mouse.down()
  await page.mouse.move(board.x + x, board.y + y, { steps: 4 })
  await page.mouse.up()
  await expect(point).toHaveAttribute('cx', String(x))
  await expect(point).toHaveAttribute('cy', String(y))
}

test('a DM takes the Default World, sets its grid and scale, and calibrates it from two points', async ({ page }, info) => {
  test.skip(info.project.name !== 'desktop', 'calibrating is desk work')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Scale ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('maps-link').click()

  await page.getByTestId('use-default-world').click()
  await expect(page.getByRole('heading', { name: 'Default World' })).toBeVisible()
  const board = page.getByTestId('map-board')
  await expect(board).toHaveAttribute('data-grid', 'hexes')
  await expect(board).toHaveAttribute('width', '1600')
  // The picture is there, painted, under the grid.
  const picture = await page.request.get(String(await page.getByTestId('map-image').getAttribute('href')))
  expect([picture.status(), picture.headers()['content-type']]).toEqual([200, 'image/png'])
  expect((await picture.body()).byteLength).toBeGreaterThan(10_000)

  // The grid is drawn see-through: its lines are white at the strength the DM sets.
  await page.getByTestId('grid-kind').selectOption('squares')
  await page.getByTestId('grid-strength').fill('60')
  await expect(page.getByTestId('grid-squares')).toHaveCSS('stroke', 'rgba(255, 255, 255, 0.6)')
  await expect(page.locator('[data-hex="0,0"] polygon').first()).toHaveCSS('stroke', 'rgba(255, 255, 255, 0)')
  await page.getByTestId('scale-miles').fill('12')
  await page.getByRole('button', { name: 'Save calibration' }).click()
  await expect(page.getByTestId('calibration-saved')).toBeVisible()

  // Two points 400 px apart that the DM knows to be 120 miles apart: ten cells of 12 miles, 40 px each.
  await dragTo(page, page.getByTestId('calibrate-a'), 500, 480)
  await dragTo(page, page.getByTestId('calibrate-b'), 900, 480)
  await page.getByTestId('calibrate-distance').fill('120')
  await page.getByTestId('calibrate-go').click()
  await expect(page.getByTestId('calibrated')).toHaveText('Calibrated: a cell is 40.0 px across.')
  await expect(page.getByTestId('calibrate-measured')).toHaveText('Now 10.0 cells, 120.0 miles apart.')
  await expect(page.getByTestId('origin-x')).toHaveValue('500')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // It is all kept.
  await page.reload()
  await expect(page.getByTestId('grid-kind')).toHaveValue('squares')
  await expect(page.getByTestId('scale-miles')).toHaveValue('12')
  await expect(page.getByTestId('grid-strength')).toHaveValue('60')
  await expect(page.getByTestId('origin-x')).toHaveValue('500')
  await expect(page.getByTestId('grid-squares')).toHaveCount(1)

  // A battle map keeps its 5 ft hexes.
  await page.getByRole('link', { name: '← Maps' }).click()
  await page.getByTestId('map-name').fill('Crypt')
  await page.getByTestId('map-file').setInputFiles({ name: 'crypt.png', mimeType: 'image/png', buffer: crypt })
  await page.getByRole('button', { name: 'Upload map' }).click()
  await expect(page.getByRole('heading', { name: 'Crypt' })).toBeVisible()
  await expect(page.getByTestId('scale-fixed')).toHaveText('A battle map keeps its hexes: each is 5 feet across.')
  await expect(page.getByTestId('grid-kind')).toHaveCount(0)
  await expect(page.getByTestId('calibrate-measured')).toContainText('feet apart')
  await expect(page.getByTestId('map-board')).toHaveAttribute('data-grid', 'hexes')

  // In play the world map shows the grid the DM chose.
  await page.getByRole('link', { name: '← Maps' }).click()
  await page.getByRole('link', { name: '← Campaign' }).click()
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await page.getByTestId('scope-world').check()
  await page.getByTestId('world-choice').selectOption({ label: 'Default World' })
  await page.getByTestId('use-world').click()
  await expect(page.getByTestId('map-board')).toHaveAttribute('data-grid', 'squares')
  await expect(page.getByTestId('grid-squares')).toHaveCSS('stroke', 'rgba(255, 255, 255, 0.6)')
})
