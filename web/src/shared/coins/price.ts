// formatPrice writes a price in copper as gold, silver and copper, as a shop would ask it.
export function formatPrice(cp: number): string {
  const parts = [
    [Math.floor(cp / 100), 'gp'],
    [Math.floor((cp % 100) / 10), 'sp'],
    [cp % 10, 'cp'],
  ] as const
  const said = parts.filter(([n]) => n > 0).map(([n, coin]) => `${String(n)} ${coin}`)
  return said.length > 0 ? said.join(' ') : '0 cp'
}

// haggled is an asking price after a haggling adjustment in percent; nothing costs less than a copper.
export function haggled(cp: number, adjustPct: number): number {
  return Math.max(1, Math.floor((cp * (100 + adjustPct)) / 100))
}
