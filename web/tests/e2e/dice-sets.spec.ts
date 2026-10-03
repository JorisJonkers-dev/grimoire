import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { account } from './accounts'

// A real picture, one pixel of it.
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64')

test('a Dice Set is designed, shared with a Friend who takes a copy, checked by an Admin, and rolled with', async ({ browser, request }, info) => {
  test.skip(!['phone', 'desktop'].includes(info.project.name), 'the Dice Sets page is checked on phone and desktop')
  test.slow()
  const stamp = `${info.project.name}-${String(Date.now())}`
  const name = `Ember ${stamp}`
  const owner = await account(browser, request, `own-${stamp}`, 'Owner', { reducedMotion: 'reduce' })
  const friend = await account(browser, request, `pal-${stamp}`, 'Pal')
  await owner.page.request.post('/api/v1/friend-requests', { data: { username: `pal-${stamp}` } })
  const asked = (await (await friend.page.request.get('/api/v1/friends')).json()) as { incoming: { id: string }[] }
  await friend.page.request.post(`/api/v1/friend-requests/${asked.incoming[0]?.id ?? ''}/accept`)

  // Designed die by die, with a picture placed on the unwrapped faces of the d20.
  const page = owner.page
  await page.goto('/dice-sets')
  await page.getByTestId('dice-set-new').click()
  await page.getByTestId('set-name').fill(name)
  await page.getByTestId('look-pattern').selectOption('stripes')
  await page.getByTestId('look-body').fill('#102030')
  await page.getByTestId('look-numbers').fill('#fafafa')
  await page.getByTestId('look-all').click()
  await page.getByTestId('set-save').click()
  const editor = page.getByTestId('dice-set-editor')
  await expect(editor.getByRole('heading')).toHaveText(`Edit ${name}`)
  await editor.getByTestId('set-picture-file').setInputFiles({ name: 'crest.png', mimeType: 'image/png', buffer: png })
  await editor.getByTestId('picture-on').check()
  await editor.getByTestId('picture-x').fill('0.25')
  await editor.getByTestId('picture-scale').fill('0.5')
  await expect(editor.getByTestId('sheet-picture')).toHaveAttribute('style', /left: 25%; top: 50%; width: 50%/)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await editor.getByTestId('set-save').click()
  await editor.getByTestId('set-done').click()
  await expect(editor).toHaveCount(0)

  // Shared with Friends, the Friend takes a copy: theirs to roll with, not to change.
  await page.getByTestId(`sharing-${name}`).selectOption('friends')
  await friend.page.goto('/dice-sets')
  await expect(friend.page.getByTestId(`shared-set-${name}`)).toContainText(`by own-${stamp}`)
  await friend.page.getByTestId(`copy-${name}`).click()
  await expect(friend.page.getByTestId(`copy-of-${name}`)).toHaveText(`a copy, by own-${stamp}`)
  await expect(friend.page.getByTestId(`edit-${name}`)).toHaveCount(0)
  await friend.page.getByTestId(`copy-${name}`).click()
  await expect(friend.page.getByTestId('dice-sets-problem')).toHaveText('You already have a copy of that set.')
  expect((await new AxeBuilder({ page: friend.page }).analyze()).violations).toEqual([])

  // Shared with everyone, its picture waits for an Admin.
  await page.getByTestId(`sharing-${name}`).selectOption('everyone')
  await expect(page.getByTestId(`review-${name}`)).toContainText('Waiting for an Admin')
  const admin = { 'X-User-Id': 'e2e-admin' }
  const waiting = (await (await request.get('/api/v1/admin/dice-sets', { headers: admin })).json()) as { items: { id: string; name: string; imageVersion: string }[] }
  const seen = waiting.items.find((s) => s.name === name)
  // A decision on a picture the Admin did not look at is refused.
  expect((await request.post(`/api/v1/admin/dice-sets/${seen?.id ?? ''}/review`, { headers: admin, data: { approve: true, picture: '0123456789ab' } })).status()).toBe(409)
  expect((await request.post(`/api/v1/admin/dice-sets/${seen?.id ?? ''}/review`, { headers: admin, data: { approve: true, picture: seen?.imageVersion } })).status()).toBe(200)
  await page.reload()
  await expect(page.getByTestId(`review-${name}`)).toContainText('approved')

  // Chosen, the owner's rolls are thrown in it.
  await page.getByTestId(`dice-choice-${name}`).check()
  await expect(page.getByTestId(`dice-choice-${name}`)).toBeChecked()
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Dice ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('Owner')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await expect(page.getByTestId('member-list')).toBeVisible()
  await page.goto(`${new URL(page.url()).pathname}/dice`)
  await page.getByTestId('roll-purpose').fill('Stealth')
  await page.getByTestId('roll-form').locator('[type="submit"]').click()
  await page.getByTestId('roll-rest').click()
  await expect(page.getByTestId('dice-2d').getByTestId('die-tint').first()).toHaveAttribute('fill', '#102030')
  await owner.context.close()
  await friend.context.close()
})
