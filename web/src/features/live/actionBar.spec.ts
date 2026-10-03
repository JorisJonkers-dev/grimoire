import { describe, expect, it } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { BAR_TILES, arrange, defaultLayout, move, pin, stow, tilesOf, unstow } from './actionBar'

const aria: LiveToken = {
  id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0,
  attacks: [
    { name: 'Longsword', toHit: 5, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d8', damageBonus: 3, damageType: 'slashing', mastery: 'sap' },
    { name: 'Longbow', toHit: 4, reachFt: 0, rangeFt: 150, longRangeFt: 600, damage: '1d8', damageBonus: 2, damageType: 'piercing' },
  ],
}
const keys = (slots: { key: string }[]) => slots.map((s) => s.key)

describe('action bar tiles', () => {
  it('lists a token\'s attacks, actions, unarmed strikes, moves, spells and summons, attacks first', () => {
    const tiles = tilesOf(aria)
    expect(tiles.slice(0, 3).map((t) => t.key)).toEqual(['attack:Longsword', 'attack:Longbow', 'action:dash'])
    expect(tiles.find((t) => t.key === 'attack:Longsword')).toMatchObject({ name: 'Longsword', testid: 'attack-0', detail: '+5 · 1d8+3 · reach 5 ft' })
    expect(tiles.find((t) => t.key === 'move:swap')).toMatchObject({ name: 'Swap weapons', testid: 'swap-weapons' })
    expect(tilesOf({ ...aria, kind: 'enemy' }).some((t) => t.key === 'move:swap')).toBe(false)
    expect(tiles.map((t) => t.key)).toEqual(expect.arrayContaining(['unarmed:grapple', 'move:jump', 'move:misty-step', 'spell:fireball', 'summon:find-familiar']))
    expect(new Set(tiles.map((t) => t.key)).size).toBe(tiles.length)
  })

  it('starts from two bars of ten in the usual order, with the first four on the quick bar', () => {
    const layout = defaultLayout(tilesOf(aria))
    expect(layout.bars.map((b) => b.length)).toEqual([BAR_TILES, BAR_TILES])
    expect(layout.bars[0]?.slice(0, 2)).toEqual(['attack:Longsword', 'attack:Longbow'])
    expect(layout.quick).toEqual(layout.bars[0]?.slice(0, 4))
    expect(layout.stowed).toHaveLength(tilesOf(aria).length - 2 * BAR_TILES)
  })

  it('keeps a tile the Character lacks greyed in its place, and puts a new one at the end', () => {
    const layout = { bars: [['attack:Glaive', 'attack:Longsword'], ['action:dash']], quick: ['attack:Glaive', 'spell:fireball'], stowed: tilesOf(aria).map((t) => t.key).filter((k) => !['attack:Longsword', 'action:dash', 'attack:Longbow', 'spell:fireball'].includes(k)) }
    const got = arrange(layout, tilesOf(aria))
    expect(got.bars[0]?.[0]).toMatchObject({ key: 'attack:Glaive', name: 'Glaive' })
    expect(got.bars[0]?.[0]?.tile).toBeUndefined()
    expect(got.bars[0]?.[1]?.tile?.name).toBe('Longsword')
    // Longbow and Fireball are new to this layout: they join the end, the first bar first.
    expect(keys(got.bars[0] ?? [])).toEqual(['attack:Glaive', 'attack:Longsword', 'attack:Longbow', 'spell:fireball'])
    expect(keys(got.bars[1] ?? [])).toEqual(['action:dash'])
    expect(got.quick.map((s) => [s.key, s.tile !== undefined])).toEqual([['attack:Glaive', false], ['spell:fireball', true]])
    expect(got.stowed.every((t) => layout.stowed.includes(t.key))).toBe(true)
    // With both bars full a new tile waits in the drawer instead.
    const full = { bars: [Array.from({ length: BAR_TILES }, (_, i) => `action:a${String(i)}`), Array.from({ length: BAR_TILES }, (_, i) => `action:b${String(i)}`)], quick: [], stowed: [] }
    const crowded = arrange(full, tilesOf(aria))
    expect(crowded.bars.map((b) => b.length)).toEqual([BAR_TILES, BAR_TILES])
    expect(crowded.stowed.map((t) => t.key)).toContain('attack:Longsword')
    expect(arrange({ bars: [], quick: [], stowed: [] }, tilesOf(aria)).bars).toHaveLength(2)
  })

  it('moves, stows, brings back and pins tiles', () => {
    const base = { bars: [['a:1', 'a:2', 'a:3'], ['b:1']], quick: ['a:1'], stowed: ['s:1'] }
    expect(move(base, 'a:3', 0, 0).bars[0]).toEqual(['a:3', 'a:1', 'a:2'])
    expect(move(base, 'a:1', 1, 1).bars).toEqual([['a:2', 'a:3'], ['b:1', 'a:1']])
    expect(move(base, 's:1', 0, 1)).toMatchObject({ bars: [['a:1', 's:1', 'a:2', 'a:3'], ['b:1']], stowed: [] })
    const crowded = { ...base, bars: [Array.from({ length: BAR_TILES }, (_, i) => `x:${String(i)}`), ['b:1']] }
    // A full bar: a tile from the other bar trades places, wherever on the bar it lands; one from the drawer waits.
    expect(move(crowded, 'b:1', 0, 0).bars).toEqual([['b:1', ...(crowded.bars[0] ?? []).slice(1)], ['x:0']])
    expect(move(crowded, 'b:1', 0, BAR_TILES).bars).toEqual([[...(crowded.bars[0] ?? []).slice(0, 9), 'b:1'], ['x:9']])
    expect(move(crowded, 's:1', 0, 0)).toEqual(crowded)
    expect(move(crowded, 'b:1', 5, 0)).toEqual(crowded)
    expect(move(crowded, 'x:9', 0, 0).bars[0]?.[0]).toBe('x:9')
    expect(stow(base, 'a:1')).toEqual({ bars: [['a:2', 'a:3'], ['b:1']], quick: [], stowed: ['s:1', 'a:1'] })
    expect(unstow(base, 's:1').bars).toEqual([['a:1', 'a:2', 'a:3', 's:1'], ['b:1']])
    expect(unstow({ ...crowded, stowed: ['s:1'] }, 's:1').bars[1]).toEqual(['b:1', 's:1'])
    const packed = { bars: [crowded.bars[0] ?? [], Array.from({ length: BAR_TILES }, (_, i) => `y:${String(i)}`)], quick: [], stowed: ['s:1'] }
    expect(unstow(packed, 's:1')).toEqual(packed)
    expect(pin(base, 'a:2').quick).toEqual(['a:1', 'a:2'])
    expect(pin(base, 'a:1').quick).toEqual([])
    expect(pin({ ...base, quick: ['q:1', 'q:2', 'q:3', 'q:4'] }, 'a:2').quick).toEqual(['q:1', 'q:2', 'q:3', 'q:4'])
  })
})
