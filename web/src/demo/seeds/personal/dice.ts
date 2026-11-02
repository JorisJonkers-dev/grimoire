import type { DiceDesign, DiceSet, DieLook } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import { personalId } from './ids'

// Dice Sets carry no uploaded picture in the demo: a picture would be fetched from the server, and the
// demo has none. So nothing waits for an Admin's review either.

export type DemoDiceSet = {
  id: string
  ownerId: string
  name: string
  design: DiceDesign
  sharing: DiceSet['sharing']
  /** The set this one is a copy of; a copy cannot be changed or shared on. */
  copyOf?: string
  /** Who made the original, as a copy keeps saying. */
  by: string
  updatedAt: string
}

const every = (look: DieLook): DiceDesign => ({ dice: { d4: look, d6: look, d8: look, d10: look, d12: look, d20: look, d100: look } })

export function seedDice() {
  const [ash, brook] = ids.players
  const diceSets: DemoDiceSet[] = [
    { id: personalId(0x501), ownerId: ash, name: 'Tidewater', design: every({ pattern: 'marble', body: '#1f4e5f', numbers: '#e8d9a8' }), sharing: 'friends', by: 'Ash', updatedAt: '2026-09-12T18:00:00Z' },
    { id: personalId(0x502), ownerId: ash, name: 'Lantern Glass', design: every({ pattern: 'speckled', body: '#f2c14e', numbers: '#2b2118' }), sharing: 'private', by: 'Ash', updatedAt: '2026-10-03T10:30:00Z' },
    { id: personalId(0x503), ownerId: brook, name: 'Ember', design: every({ pattern: 'stripes', body: '#8c2f1b', numbers: '#fbe3c2' }), sharing: 'friends', by: 'Brook', updatedAt: '2026-08-20T16:00:00Z' },
    { id: personalId(0x504), ownerId: ids.outsider, name: 'Moss and Stone', design: every({ pattern: 'speckled', body: '#4f6b3a', numbers: '#f4f1e8' }), sharing: 'everyone', by: 'Fennel', updatedAt: '2026-07-04T09:00:00Z' },
    { id: personalId(0x505), ownerId: ids.dm, name: 'Drowned Brass', design: every({ pattern: 'plain', body: '#a67c37', numbers: '#14202b' }), sharing: 'private', by: 'Morgan', updatedAt: '2026-06-18T21:00:00Z' },
  ]
  /** The set each Account rolls with; one without an entry rolls plain dice. */
  const chosenDice: Record<string, string> = { [ash]: personalId(0x501), [ids.dm]: personalId(0x505) }
  return { diceSets, chosenDice }
}
