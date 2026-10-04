import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

type Rgb = [number, number, number]
type Matrix = [Rgb, Rgb, Rgb]
// How each kind of colour blindness sees linear RGB (Machado, Oliveira and Fernandes, 2009).
const sight: Record<string, Matrix> = {
  protanopia: [[0.152286, 1.052583, -0.204868], [0.114503, 0.786281, 0.099216], [-0.003882, -0.048116, 1.051998]],
  deuteranopia: [[0.367322, 0.860646, -0.227968], [0.280085, 0.672501, 0.047413], [-0.01182, 0.04294, 0.968881]],
  tritanopia: [[1.255528, -0.076749, -0.178779], [-0.078411, 0.930809, 0.147602], [0.004733, 0.691367, 0.3039]],
}
const linear = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
const rgb = (hex: string): Rgb => [linear(parseInt(hex.slice(1, 3), 16) / 255), linear(parseInt(hex.slice(3, 5), 16) / 255), linear(parseInt(hex.slice(5, 7), 16) / 255)]
const seen = (c: Rgb, m: Matrix) => m.map((row) => Math.min(1, Math.max(0, row[0] * c[0] + row[1] * c[1] + row[2] * c[2]))) as Rgb
function lab([r, g, b]: Rgb): Rgb {
  const f = (t: number) => (t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116)
  const x = f((0.4124 * r + 0.3576 * g + 0.1805 * b) / 0.95047)
  const y = f(0.2126 * r + 0.7152 * g + 0.0722 * b)
  const z = f((0.0193 * r + 0.1192 * g + 0.9505 * b) / 1.08883)
  return [116 * y - 16, 500 * (x - y), 200 * (y - z)]
}
/** How far apart two colours look to somebody with that sight: under 25 or so they are easily confused. */
function apart(a: string, b: string, m: Matrix): number {
  const [p, q] = [lab(seen(rgb(a), m)), lab(seen(rgb(b), m))]
  return Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2])
}
const light = ([r, g, b]: Rgb) => 0.2126 * r + 0.7152 * g + 0.0722 * b
function contrast(a: string, b: string): number {
  const [hi, lo] = [light(rgb(a)), light(rgb(b))].sort((x, y) => y - x) as [number, number]
  return (hi + 0.05) / (lo + 0.05)
}

const read = (file: string) => readFileSync(resolve(__dirname, file), 'utf8')
/** The colours a rule sets, by token name. */
function colours(css: string, selector: string): Record<string, string> {
  const at = css.indexOf(`${selector} {`)
  expect(at, selector).toBeGreaterThanOrEqual(0)
  const body = css.slice(at, css.indexOf('}', at))
  return Object.fromEntries([...body.matchAll(/--color-([a-z0-9-]+): (#[0-9A-Fa-f]{6});/g)].map((m) => [m[1] ?? '', m[2] ?? ''] as const))
}
const standard = colours(read('../tokens.css'), ':root')
const palette = (name: string) => ({ ...standard, ...colours(read('accessibility.css'), `:root[data-palette="${name}"]`) })
// Whose side a creature is on and whether something went well are told by colour.
const told = ['party', 'enemy', 'success'] as const
const pairs = told.flatMap((a, i) => told.slice(i + 1).map((b) => [a, b] as const))
const closest = (p: Record<string, string>, m: Matrix) => Math.min(...pairs.map(([a, b]) => apart(p[a] ?? '', p[b] ?? '', m)))

describe('colour-blind-safe palettes', () => {
  it('each palette keeps the colours further apart than the standard ones, for the sight it is made for', () => {
    for (const [name, kinds] of [['red-green', ['protanopia', 'deuteranopia']], ['blue-yellow', ['tritanopia']]] as const) {
      for (const kind of kinds) expect(closest(palette(name), sight[kind] as Matrix), kind).toBeGreaterThan(closest(standard, sight[kind] as Matrix))
    }
    // Red and green are the pair the standard colours leave too close.
    expect(closest(standard, sight.protanopia as Matrix)).toBeLessThan(30)
  })

  it.each([
    ['red-green', ['protanopia', 'deuteranopia']],
    ['blue-yellow', ['tritanopia']],
  ])('the %s palette keeps the colours that tell things apart well apart', (name, kinds) => {
    const p = palette(name)
    expect(Object.keys(colours(read('accessibility.css'), `:root[data-palette="${name}"]`)).sort()).toEqual(['enemy', 'party', 'success'])
    for (const kind of kinds) expect(closest(p, sight[kind] as Matrix), kind).toBeGreaterThanOrEqual(40)
    // And to somebody who sees every colour.
    expect(closest(p, [[1, 0, 0], [0, 1, 0], [0, 0, 1]])).toBeGreaterThanOrEqual(40)
  })

  it.each(['red-green', 'blue-yellow'])('the %s palette reads on every ground it is put on', (name) => {
    const p = palette(name)
    for (const c of told) {
      for (const ground of ['ground', 'surface', 'raised']) expect(contrast(p[c] ?? '', p[ground] ?? ''), `${c} on ${ground}`).toBeGreaterThanOrEqual(4.5)
    }
    expect(contrast(p.party ?? '', p['party-fill'] ?? '')).toBeGreaterThanOrEqual(4.5)
    expect(contrast(p.enemy ?? '', p['enemy-fill'] ?? '')).toBeGreaterThanOrEqual(4.5)
  })
})
