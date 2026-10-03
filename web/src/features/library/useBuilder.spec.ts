import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, nextTick, ref, type Ref } from 'vue'
import type { Problem } from '@/infrastructure/api/types.gen'
import { useBuilder } from './useBuilder'

type Design = { size: number }
type Build = { design: Design; entry?: { id: string; kind: 'item'; name: string; fields: []; revision: number; createdAt: string; updatedAt: string; shared?: boolean } }

function harness(data: Ref<Build | undefined>) {
  const calls: { kind: string; variables: unknown }[] = []
  const mutation = (kind: string, reply: Build) => ({
    mutate: (variables: unknown, options: { onSuccess: (b: Build) => void }) => {
      calls.push({ kind, variables })
      options.onSuccess(reply)
    },
    error: ref<Problem | null>(null),
    isPending: ref(false),
  })
  let api!: ReturnType<typeof useBuilder<Design, Build>>
  const Probe = defineComponent({
    setup() {
      api = useBuilder<Design, Build>({
        entryId: ref('e1'), data, key: ref(['k']), preview: mutation('preview', { design: { size: 2 } }), save: mutation('save', { design: { size: 3 } }), fallback: 'Thing',
      })
      return () => null
    },
  })
  mount(Probe, { global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient() }]] } })
  return { api, calls }
}

describe('useBuilder', () => {
  it('does nothing until a design has loaded, then previews under a fallback name and saves without an entry', async () => {
    const data = ref<Build>()
    const { api, calls } = harness(data)
    api.runPreview()
    api.runSave()
    expect(calls).toEqual([])
    expect(api.name.value).toBe('')
    expect(api.readOnly.value).toBe(false)
    data.value = { design: { size: 1 } }
    await nextTick()
    expect(api.design.value).toEqual({ size: 1 })
    data.value = { design: { size: 9 } }
    await nextTick()
    expect(api.design.value).toEqual({ size: 1 })
    api.runPreview()
    expect(calls[0]).toEqual({ kind: 'preview', variables: { body: { name: 'Thing', design: { size: 1 } } } })
    expect(api.shown.value?.design).toEqual({ size: 2 })
    api.runSave()
    expect(calls[1]).toEqual({ kind: 'save', variables: { path: { entryId: 'e1' }, body: { size: 1 } } })
    expect(api.status.value).toBe('Saved as Revision 0.')
  })
})
