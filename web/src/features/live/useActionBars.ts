import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { getActionBarsOptions, getActionBarsQueryKey, listMyCharactersOptions, setActionBarsMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Layout } from './actionBar'

/**
 * The bar layout of the Character a token was placed from, for the player who owns it. The layout
 * belongs to the Character, so the token's Campaign Character is first matched to it.
 */
export function useActionBars(campaignCharacterId: () => string | undefined, own: () => boolean) {
  const client = useQueryClient()
  const wanted = computed(() => own() && campaignCharacterId() !== undefined)
  const mine = useQuery(computed(() => ({ ...listMyCharactersOptions(), enabled: wanted.value })))
  const characterId = computed(() => (wanted.value ? mine.data.value?.items.find((o) => o.campaigns.some((c) => c.characterId === campaignCharacterId()))?.id : undefined))
  const options = computed(() => ({ path: { characterId: characterId.value ?? '' } }))
  const stored = useQuery(computed(() => ({ ...getActionBarsOptions(options.value), enabled: characterId.value !== undefined })))
  const saving = useMutation(setActionBarsMutation())
  // What the player last chose shows at once; the server's answer follows.
  const chosen = ref<Layout>()

  const available = computed(() => characterId.value !== undefined && stored.data.value !== undefined)
  const layout = computed<Layout | undefined>(() => {
    const s = stored.data.value
    return chosen.value ?? (s?.arranged ? { bars: s.bars, quick: s.quick, stowed: s.stowed } : undefined)
  })
  function save(next: Layout) {
    chosen.value = next
    saving.mutate({ ...options.value, body: next }, { onSuccess: (saved) => client.setQueryData(getActionBarsQueryKey(options.value), saved) })
  }
  return { available, layout, save }
}
