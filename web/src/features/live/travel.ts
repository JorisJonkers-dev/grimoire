import type { LiveTravelLeg } from '@/infrastructure/api/types.gen'

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
