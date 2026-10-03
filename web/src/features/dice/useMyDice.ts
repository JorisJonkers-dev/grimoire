import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { listDiceSetsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { lookOf } from './sets'

/** The look of the Dice Set the signed-in Account rolls with; nothing on the plain dice or without an Account. */
export function useMyDice() {
  const sets = useQuery({ ...listDiceSetsOptions(), retry: false, staleTime: 60_000 })
  return computed(() => {
    const chosen = sets.data.value?.items.find((s) => s.id === sets.data.value?.chosen)
    return chosen ? lookOf(chosen) : undefined
  })
}
