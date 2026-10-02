import { expect, test } from '@playwright/test'
import { buildFighter } from './wizard'

const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAEklEQVR4nGP4z8DAwMDAwMAAAB0ABvS+bjsAAAAASUVORK5CYII=',
  'base64',
)

test('a player uploads a portrait and crops a token icon from it', async ({ page }, info) => {
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Portraits ${info.project.name} ${String(Date.now())}`)
  await page.getByTestId('campaign-display-name').fill('Painter')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('build-character').click()
  await buildFighter(page, 'Mira')
  await page.getByTestId('create-character').click()

  if (info.project.name === 'phone') await page.getByTestId('part-gear').click()
  await page.getByTestId('portrait-file').setInputFiles({ name: 'mira.png', mimeType: 'image/png', buffer: png })
  await expect(page.getByTestId('portrait')).toBeVisible()
  const editor = page.getByTestId('token-editor')
  await editor.getByRole('radio', { name: 'Crop from the portrait' }).check()
  await expect(editor.getByTestId('save-token')).toBeEnabled()
  await editor.getByTestId('save-token').click()
  await expect(page.getByTestId('sheet-token').getByTestId('token-icon')).toHaveAttribute('src', /\/token\?v=/)

  await page.getByRole('link', { name: '← Campaign' }).click()
  await expect(page.getByTestId('party').getByTestId('token-icon')).toHaveAttribute('src', /\/token\?v=/)
})
