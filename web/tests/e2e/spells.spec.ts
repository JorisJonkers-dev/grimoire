import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

const wizard = {
  name: 'Mira', species: 'human', class: 'wizard', background: 'sage', method: 'point-buy',
  base: { strength: 8, dexterity: 14, constitution: 13, intelligence: 15, wisdom: 12, charisma: 8 },
  bonus: { intelligence: 2, constitution: 1 }, skills: ['investigation', 'medicine'], armor: '', shield: false, weapons: [],
}

test('a wizard copies spells into the spellbook, prepares them and casts a ritual', async ({ page }, info) => {
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Spells ${info.project.name} ${String(Date.now())}`)
  await page.getByTestId('campaign-display-name').fill('Mira')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await expect(page.getByTestId('build-character')).toBeVisible()
  const campaign = new URL(page.url()).pathname.split('/')[2] ?? ''
  const created = await page.request.post(`/api/v1/campaigns/${campaign}/characters`, { data: wizard })
  expect(created.status()).toBe(201)
  const id = ((await created.json()) as { id: string }).id

  await page.goto(`/campaigns/${campaign}/characters/${id}`)
  await page.getByTestId('open-spells').click()
  const book = page.getByTestId('spellbook')
  await expect(book).toContainText('Spellbook · 0 spells')
  for (const spell of ['find-familiar', 'magic-missile']) {
    await book.getByTestId('copy-spell').selectOption(spell)
    await book.getByTestId('copy-submit').click()
    await expect(page.getByTestId('spells-status')).toHaveText('Copied into the spellbook.')
  }
  await expect(book).toContainText('Spellbook · 2 spells')
  await expect(book.getByTestId('copy-cost')).toHaveText('Free: 4 of 6 left for this level.')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  const prepare = page.getByTestId('prepare-wizard')
  await prepare.getByTestId('prep-magic-missile').check()
  await prepare.getByTestId('prepare-save-wizard').click()
  await expect(page.getByTestId('spells-status')).toHaveText('Wizard spells prepared.')
  await expect(page.getByTestId('prepared')).toContainText('Magic Missile')

  await page.getByTestId('ritual-find-familiar').click()
  await expect(page.getByTestId('spells-status')).toContainText('Cast Find Familiar as a ritual: 70 minutes. It is now Day 0, 01:10.')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
