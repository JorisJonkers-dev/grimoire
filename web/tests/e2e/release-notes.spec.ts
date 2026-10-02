import { randomInt } from 'node:crypto'
import { expect, test } from '@playwright/test'

test('a published Release Note shows once on the Dashboard', async ({ browser, request }, info) => {
  test.skip(!['phone', 'desktop'].includes(info.project.name), 'the Dashboard card is checked on phone and desktop')
  const admin = { 'X-User-Id': 'e2e-admin' }
  const unique = String(randomInt(1, 2_000_000_000))
  const version = `9.${String(info.project.name.length)}.${unique}`
  const drafted = await request.post('/api/v1/admin/release-notes', { headers: admin, data: { version } })
  expect(drafted.status()).toBe(201)
  const { id } = (await drafted.json()) as { id: string }
  await request.put(`/api/v1/admin/release-notes/${id}`, { headers: admin, data: { title: `Grimoire ${version}`, body: '- Friends\n- Release Notes' } })
  expect((await request.post(`/api/v1/admin/release-notes/${id}/publish`, { headers: admin, data: {} })).status()).toBe(200)

  const created = await request.post('/api/v1/admin/account-invites', { headers: admin, data: { hours: 24 } })
  const { token } = (await created.json()) as { token: string }
  const username = `e2e-rn-${unique}`
  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto(`/account-invite#${token}`)
  await page.getByTestId('setup-username').fill(username)
  await page.getByTestId('setup-nickname').fill('Reader')
  await page.getByTestId('setup-email').fill(`${username}@example.com`)
  await page.getByTestId('setup-password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Create my Account' }).click()
  await expect(page.getByTestId('account-link')).toHaveText('Reader')

  await page.goto('/')
  // Other runs publish their own notes; dismiss newer ones until this run's shows, then it never returns.
  const card = page.getByTestId('release-note')
  await expect(card).toBeVisible()
  let found = false
  for (let i = 0; i < 10 && (await card.count()) > 0; i++) {
    if ((await card.textContent())?.includes(`Grimoire ${version}`)) {
      found = true
      await expect(card.getByRole('listitem')).toHaveText(['Friends', 'Release Notes'])
    }
    await Promise.all([
      page.waitForResponse((r) => r.url().includes('/api/v1/release-notes/unseen')),
      page.getByTestId('release-note-seen').click(),
    ])
  }
  expect(found).toBe(true)
  const [after] = await Promise.all([page.waitForResponse((r) => r.url().includes('/api/v1/release-notes/unseen')), page.reload()])
  const still = (await after.json()) as { note?: { version: string } }
  expect(still.note?.version).not.toBe(version)
  await context.close()
})
