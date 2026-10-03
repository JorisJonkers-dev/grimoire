import { ref, shallowRef } from 'vue'
import type { ShownRoll } from './choreography'

// One stage for the whole tab: whatever resolves a roll throws it here, and the page shows it.
const roll = shallowRef<ShownRoll | null>(null)
const n = ref(0)
let last = ''

/**
 * Throws a resolved roll on this screen's dice stage. The roller's own screen hears of its roll twice,
 * from the Roll Card and from the table, so a roll already thrown is not thrown again.
 */
export function throwDice(shown: ShownRoll, id: string) {
  if (id === last) return
  last = id
  roll.value = shown
  n.value++
}

export function useDiceStage() {
  return { roll, n }
}
