import type { LiveCheck } from '@/infrastructure/api/types.gen'

const triggers = { short_rest: 'Short rest', long_rest: 'Long rest', travel_leg: 'Travel leg', dm: 'The DM checks' } as const

/** One check as a line: what set it off, the open roll, and the outcome, with the DM's detail when sent. */
export function checkLine(c: LiveCheck): string {
  const parts = [c.tableName ? `${triggers[c.trigger]} on ${c.tableName}` : triggers[c.trigger]]
  if (c.chanceRoll) parts.push(`rolled ${String(c.chanceRoll)} against ${String(c.chancePct ?? 0)}%`)
  if (c.status === 'pending') parts.push('rolling…')
  else if (c.outcome === 'encounter') parts.push(c.entryLabel ? `encounter: ${c.entryLabel}` : 'something approaches!')
  else parts.push(c.entryLabel ? `all quiet (${c.entryLabel})` : 'all quiet')
  if (c.monsters?.length) parts.push(c.monsters.map((m) => (m.count === 1 ? m.slug : `${m.slug} x${String(m.count)}`)).join(', '))
  if (c.seed) parts.push(`seed ${c.seed}`)
  return parts.join(' · ')
}
