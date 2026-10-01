import AxeBuilder from '@axe-core/playwright'
import { type APIRequestContext, expect, test } from '@playwright/test'

// An MCP client over Streamable HTTP, speaking JSON-RPC the way an agent's connector does.
async function connect(request: APIRequestContext) {
  const headers = { Accept: 'application/json, text/event-stream', 'Content-Type': 'application/json' }
  const init = await request.post('/mcp', {
    headers,
    data: { jsonrpc: '2.0', id: 1, method: 'initialize', params: { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'e2e-agent', version: '1' } } },
  })
  expect(init.status()).toBe(200)
  const session = { ...headers, 'Mcp-Session-Id': init.headers()['mcp-session-id'] ?? '' }
  expect((await request.post('/mcp', { headers: session, data: { jsonrpc: '2.0', method: 'notifications/initialized' } })).status()).toBe(202)
  let id = 1
  return async (name: string, args: object) => {
    id += 1
    const res = await request.post('/mcp', { headers: session, data: { jsonrpc: '2.0', id, method: 'tools/call', params: { name, arguments: args } } })
    const body = (await res.json()) as { result: { isError?: boolean; content: { text: string }[] } }
    return { failed: body.result.isError === true, text: body.result.content[0]?.text ?? '' }
  }
}

test('an agent preps an NPC through MCP and the DM undoes it from the activity page', async ({ page }, info) => {
  test.skip(info.project.name !== 'desktop', 'one run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Agent ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await expect(page.getByTestId('activity-link')).toBeVisible()
  const campaignId = new URL(page.url()).pathname.split('/')[2] ?? ''

  const call = await connect(page.request)
  const made = await call('create_npc', { campaignId, body: { name: 'Tamsin', title: 'Innkeeper', disposition: 'friendly' } })
  expect(made.failed).toBe(false)
  expect(made.text).toContain('"revision":{')
  expect(made.text).toContain('"no":1,"action":"create"')
  const missing = await call('update_npc', { campaignId, body: { name: 'Tamsin', disposition: 'hostile' } })
  expect(missing).toEqual({ failed: true, text: 'Missing argument npcId.' })

  await page.getByTestId('activity-link').click()
  const entry = page.getByTestId('activity-Tamsin-1')
  await expect(entry).toContainText('e2e-agent created the NPC Tamsin')
  await expect(page.getByTestId('mcp-url')).toHaveValue(/\/mcp$/)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('undo-Tamsin-1').click()
  await expect(entry).toContainText('changed since')
  await expect(page.getByTestId('undo-Tamsin-1')).toHaveCount(0)

  const listed = await call('list_npcs', { campaignId })
  expect(listed).toEqual({ failed: false, text: '{"result":[]}' })
})

test('an agent spawns an encounter in a live session and the DM undoes it from the Action Log', async ({ page }, info) => {
  test.skip(info.project.name !== 'desktop', 'one run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Live agent ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const [, , campaignId, , sessionId] = new URL(page.url()).pathname.split('/')

  const call = await connect(page.request)
  const spawned = await call('spawn_encounter', { campaignId, sessionId, body: { monsters: [{ monsterSlug: 'goblin-warrior', count: 2 }], q: 0, r: 0 } })
  expect(spawned.failed).toBe(false)
  const actionSeq = (JSON.parse(spawned.text) as { result: { actionSeq: number } }).result.actionSeq
  await expect(page.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Goblin Warrior 1/)
  const entry = page.getByTestId(`log-${String(actionSeq)}`)
  await expect(entry).toContainText('encounter spawned: Goblin Warrior 1, Goblin Warrior 2')
  await expect(entry).toContainText('via e2e-agent')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  await page.getByTestId(`undo-${String(actionSeq)}`).click()
  await expect(page.locator('[data-hex="0,0"]')).not.toHaveAttribute('aria-label', /Goblin/)
  await expect(page.getByTestId(`undo-${String(actionSeq)}`)).toHaveCount(0)
  const again = await call('undo_action', { campaignId, sessionId, body: { seq: actionSeq } })
  expect(again).toEqual({ failed: true, text: 'That action is already undone.' })
})
