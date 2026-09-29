export type Segment = { text: string; mention?: string }

/** Splits text into plain runs and runs naming one of the terms (whole words, any case). */
export function highlight(text: string, terms: readonly string[]): Segment[] {
  const usable = terms.filter((t) => t.trim() !== '')
  if (usable.length === 0) return [{ text }]
  const escaped = usable.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).sort((a, b) => b.length - a.length)
  const pattern = new RegExp(`\\b(${escaped.join('|')})\\b`, 'gi')
  const segments: Segment[] = []
  let last = 0
  for (const match of text.matchAll(pattern)) {
    const start = match.index
    if (start > last) segments.push({ text: text.slice(last, start) })
    const found = match[0]
    const term = usable.find((t) => t.toLowerCase() === found.toLowerCase()) ?? found
    segments.push({ text: found, mention: term })
    last = start + found.length
  }
  if (last < text.length) segments.push({ text: text.slice(last) })
  return segments
}

export const schools = ['abjuration', 'conjuration', 'divination', 'enchantment', 'evocation', 'illusion', 'necromancy', 'transmutation'] as const
export const classes = ['bard', 'cleric', 'druid', 'paladin', 'ranger', 'sorcerer', 'warlock', 'wizard'] as const

export function levelLabel(level: number): string {
  if (level === 0) return 'Cantrip'
  const suffix = level === 1 ? 'st' : level === 2 ? 'nd' : level === 3 ? 'rd' : 'th'
  return `${level}${suffix} level`
}

export function titleCase(slug: string): string {
  return slug
    .split('-')
    .map((p) => p.charAt(0).toUpperCase() + p.slice(1))
    .join(' ')
}
