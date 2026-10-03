import { useQueryClient, type QueryKey } from '@tanstack/vue-query'
import { computed, ref, watch, type Ref } from 'vue'
import type { LibraryEntry, Problem } from '@/infrastructure/api/types.gen'

type Built<D> = { design: D; entry?: LibraryEntry }
type Mutation<B, V> = {
  mutate: (variables: V, options: { onSuccess: (built: B) => void }) => void
  error: Ref<Problem | null>
  isPending: Ref<boolean>
}

/**
 * What every homebrew builder shares: the design being edited, copied out of the entry it opens; the
 * build last shown; previewing the design without saving it; and saving it as the entry's next Revision.
 */
export function useBuilder<D, B extends Built<D>>(kit: {
  entryId: Ref<string>
  data: Ref<B | undefined>
  key: Ref<QueryKey>
  preview: Mutation<B, { body: { name: string; design: D } }>
  save: Mutation<B, { path: { entryId: string }; body: D }>
  fallback: string
}) {
  const client = useQueryClient()
  // copy takes plain data out of a reactive value.
  const copy = <T>(v: T): T => JSON.parse(JSON.stringify(v)) as T
  const design = ref<D>()
  const shown = ref<B>()
  const status = ref('')
  watch(
    kit.data,
    (b) => {
      if (!b || design.value !== undefined) return
      design.value = copy(b.design)
      shown.value = b
    },
    { immediate: true },
  )
  const name = computed(() => kit.data.value?.entry?.name ?? '')
  const readOnly = computed(() => kit.data.value?.entry?.shared ?? false)
  const problem = computed(() => kit.preview.error.value ?? kit.save.error.value)

  function runPreview() {
    if (design.value === undefined) return
    status.value = ''
    kit.preview.mutate({ body: { name: name.value || kit.fallback, design: design.value } }, { onSuccess: (b) => (shown.value = b) })
  }
  function runSave() {
    if (design.value === undefined) return
    status.value = ''
    kit.save.mutate({ path: { entryId: kit.entryId.value }, body: design.value }, {
      onSuccess: (b) => {
        shown.value = b
        client.setQueryData(kit.key.value, b)
        void client.invalidateQueries()
        status.value = `Saved as Revision ${String(b.entry?.revision ?? 0)}.`
      },
    })
  }
  return { design, shown, status, name, readOnly, problem, runPreview, runSave }
}
