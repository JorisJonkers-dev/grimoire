import type { LiveTableResult } from '@/infrastructure/api/types.gen'

/** What a roll on a Roll Table landed on, as one line: who rolled what on which table, and what happens. */
export function tableResultLine(r: LiveTableResult): string {
  const on = r.table ? ` on ${r.table}` : ''
  return `${r.hook}: ${r.label} rolled ${String(r.total)}${on}. ${r.text}`
}
