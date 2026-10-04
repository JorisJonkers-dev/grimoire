import { criticalOf, type ShownRoll } from '@/features/dice/choreography'

/** How many lines the log a screen reader follows keeps. */
export const KEPT_LINES = 6

const list = new Intl.ListFormat('en-GB', { type: 'conjunction' })

/** What a screen reader is told as turns start: mine by name, anybody else's by who acts. */
export function turnLine(round: number, names: string[], mine: string[]): string {
  const said = `Round ${String(round)}.`
  if (mine.length > 0) return `${said} Your turn: ${list.format(mine)}.`
  if (names.length === 0) return `${said} A creature acts.`
  return `${said} ${list.format(names)} ${names.length === 1 ? 'acts' : 'act'}.`
}

const naturals = { hit: ', a natural 20', miss: ', a natural 1' }

/** What a screen reader is told as a roll lands. */
export function rollLine(roll: ShownRoll): string {
  const critical = criticalOf(roll)
  return `${roll.roller} rolled ${String(roll.total)} for ${roll.purpose}${critical ? naturals[critical] : ''}.`
}
