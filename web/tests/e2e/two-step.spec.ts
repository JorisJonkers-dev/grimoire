import { createHmac } from 'node:crypto'
import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

// RFC 6238 with SHA-1, six digits and 30-second steps, as an authenticator app computes it.
function totp(secret: string, at: number) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  let bits = ''
  for (const ch of secret) bits += alphabet.indexOf(ch).toString(2).padStart(5, '0')
  const key = Buffer.from((bits.match(/.{8}/g) ?? []).map((b) => parseInt(b, 2)))
  const msg = Buffer.alloc(8)
  msg.writeBigUInt64BE(BigInt(Math.floor(at / 30000)))
  const sum = createHmac('sha1', key).update(msg).digest()
  const off = (sum.at(-1) ?? 0) & 0x0f
  return String((sum.readUInt32BE(off) & 0x7fffffff) % 1_000_000).padStart(6, '0')
}

test('two-step sign-in asks for one numeric code after the password', async ({ browser, request }, info) => {
  test.skip(!['phone', 'desktop'].includes(info.project.name), 'the code page is checked on phone and desktop')
  const created = await request.post('/api/v1/admin/account-invites', { headers: { 'X-User-Id': 'e2e-admin' }, data: { hours: 24 } })
  const { token } = (await created.json()) as { token: string }
  const username = `e2e-2s-${info.project.name}-${String(Date.now())}`

  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto(`/account-invite#${token}`)
  await page.getByTestId('setup-username').fill(username)
  await page.getByTestId('setup-nickname').fill('Odile')
  await page.getByTestId('setup-email').fill(`${username}@example.com`)
  await page.getByTestId('setup-password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Create my Account' }).click()
  await expect(page.getByTestId('account-link')).toHaveText('Odile')

  await page.goto('/account')
  await page.getByTestId('two-step-begin').click()
  await expect(page.getByRole('img', { name: 'Two-step sign-in QR code' })).toBeVisible()
  const secret = (await page.getByTestId('two-step-secret').textContent()) ?? ''
  await page.getByTestId('two-step-confirm-code').fill(totp(secret, Date.now()))
  await page.getByRole('button', { name: 'Turn on two-step' }).click()
  await expect(page.getByTestId('recovery-codes').locator('li')).toHaveCount(10)

  await page.getByTestId('sign-out').click()
  await page.getByTestId('sign-in-username').fill(username)
  await page.getByTestId('sign-in-password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Sign in' }).click()
  const code = page.getByTestId('two-step-code')
  await expect(code).toHaveAttribute('inputmode', 'numeric')
  await expect(code).toHaveAttribute('autocomplete', 'one-time-code')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await code.fill('000000')
  await page.getByRole('button', { name: 'Continue' }).click()
  await expect(page.getByTestId('two-step-wrong')).toBeVisible()
  // The code from the step after the one that turned two-step on: never a reused code.
  await code.fill(totp(secret, Date.now() + 30000))
  await page.getByRole('button', { name: 'Continue' }).click()
  await expect(page.getByTestId('account-link')).toHaveText('Odile')
  await context.close()
})
