import type { LiveMeasure, LiveTravelLeg, TravelPace } from '@/infrastructure/api/types.gen'

// How long a journey takes, as the table says it: hours on the road, and days when it spans several.
export function duration(minutes: number, days: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  const time = h === 0 ? `${String(m)} min` : m === 0 ? `${String(h)} h` : `${String(h)} h ${String(m)} min`
  return days > 1 ? `${String(days)} days (${time} on the road)` : time
}

// The whole journey so far: miles, and time on the road with the travel days it took.
export function journey(legs: LiveTravelLeg[]): string {
  const miles = legs.reduce((n, l) => n + l.distanceMi, 0)
  const minutes = legs.reduce((n, l) => n + l.minutes, 0)
  const days = legs.reduce((n, l) => n + l.days, 0)
  return `${String(miles)} mi · ${duration(minutes, days)}`
}

/** A route may be measured through this many points. */
export const MAX_WAYPOINTS = 50

const count = (n: number, one: string) => `${String(n)} ${one}${n === 1 ? '' : one === 'hex' ? 'es' : 's'}`

// A measured route as the table says it: hexes, miles, and the time on the road at one pace.
export function measured(m: LiveMeasure, pace: TravelPace): string {
  const length = `${count(m.hexes, 'hex')} · ${count(Number(m.miles.toFixed(1)), 'mile')}`
  const plan = m.plans.find((p) => p.pace === pace)
  return plan ? `${length} · ${duration(plan.minutes, plan.days)} at a ${pace} pace` : length
}

/** Dawn on the Game Clock, in minutes after midnight. */
export const DAWN_MINUTE = 6 * 60
const two = (n: number) => String(n).padStart(2, '0')

// The Game Clock as the table reads it: the day and the time of day.
export const clockText = (day: number, minute: number) => `Day ${String(day)}, ${two(Math.floor(minute / 60))}:${two(minute % 60)}`

/** The next dawn after a time on the Game Clock. */
export const nextDawn = (day: number, minute: number) => ({ gameDay: minute < DAWN_MINUTE ? day : day + 1, gameMinute: DAWN_MINUTE })
