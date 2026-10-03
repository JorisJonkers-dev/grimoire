import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('a DM keeps a creature in the Library, links it, overrides it and pins a Revision', async ({ page }, info) => {
  const stamp = `${info.project.name} ${String(Date.now())}`
  const campaign = `Library ${stamp}`
  const hag = `Bog Hag ${stamp}`
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(campaign)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await expect(page.getByTestId('library-link')).toBeVisible()

  await page.goto('/library')
  await page.getByTestId('library-kind').selectOption('creature')
  await page.getByTestId('library-name').fill(hag)
  for (const [i, [name, value]] of [['HP', '52'], ['AC', '17']].entries()) {
    await page.getByTestId('field-add').click()
    await page.getByTestId(`field-name-${String(i)}`).fill(name ?? '')
    await page.getByTestId(`field-value-${String(i)}`).fill(value ?? '')
  }
  await page.getByTestId('library-add').click()
  await expect(page.getByRole('heading', { level: 1 })).toContainText(hag)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('entry-link-target').selectOption({ label: campaign })
  await page.getByTestId('entry-link').click()
  await expect(page.getByTestId('entry-uses')).toContainText(`${campaign} · follows the latest`)

  await page.getByTestId('entry-uses').getByRole('link', { name: campaign }).click()
  const card = page.getByTestId(`linked-${hag}`)
  await expect(card.getByTestId('value-HP')).toHaveText('52')
  await card.getByTestId('override-edit').click()
  await card.getByTestId('field-add').click()
  await card.getByTestId('field-name-0').fill('HP')
  await card.getByTestId('field-value-0').fill('30')
  await card.getByTestId('override-save').click()
  await expect(card.getByTestId('value-HP')).toContainText('30')
  await expect(card.getByTestId('value-HP')).toContainText('Campaign Override')
  await card.getByTestId('pin').selectOption('1')
  await expect(card.getByTestId('pin')).toHaveValue('1')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.goto('/library')
  await page.getByRole('link', { name: hag }).click()
  await page.getByTestId('field-value-0').fill('18')
  await page.getByTestId('entry-save').click()
  await expect(page.getByTestId('entry-status')).toHaveText('Saved as Revision 2.')
  await expect(page.getByTestId('entry-uses')).toContainText(`${campaign} · pinned to Revision 1`)

  await page.getByTestId('entry-uses').getByRole('link', { name: campaign }).click()
  await expect(card.getByTestId('value-AC')).toHaveText('17')
  await card.getByTestId('pin').selectOption('')
  await expect(card.getByTestId('value-AC')).toHaveText('18')
  await expect(card.getByTestId('value-HP')).toContainText('30')
})
