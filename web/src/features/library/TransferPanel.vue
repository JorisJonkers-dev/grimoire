<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { importLibraryMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { exportLibrary } from '@/infrastructure/api/sdk.gen'
import type { ImportReport, LibraryImport } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const client = useQueryClient()
const take = useMutation(importLibraryMutation())
const report = ref<ImportReport>()
const problem = ref('')

// download saves the caller's Homebrew, all of it or one Collection, as a JSON file.
async function download(collectionId?: string) {
  problem.value = ''
  const { data } = await exportLibrary({ query: collectionId ? { collectionId } : {} })
  if (!data) {
    problem.value = 'The export could not be made.'
    return
  }
  const link = document.createElement('a')
  link.href = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }))
  link.download = 'grimoire-library.json'
  link.click()
  URL.revokeObjectURL(link.href)
}
defineExpose({ download })

async function upload(ev: Event) {
  problem.value = ''
  report.value = undefined
  const file = (ev.target as HTMLInputElement).files?.[0]
  if (!file) return
  let body: LibraryImport
  try {
    body = JSON.parse(await file.text()) as LibraryImport
  } catch {
    problem.value = 'That file is not JSON.'
    return
  }
  take.mutate({ body }, {
    onSuccess: (r) => {
      report.value = r
      void client.invalidateQueries()
    },
    onError: () => {
      problem.value = 'That file is not a Grimoire Library export.'
    },
  })
}
</script>

<template>
  <section class="g-card stack" aria-label="Import and export" data-testid="transfer">
    <h2>Import and export</h2>
    <p class="hint">Back up or share your Homebrew as JSON in Grimoire's own schema.</p>
    <div class="row">
      <GButton data-testid="export-all" @click="download()">Export everything</GButton>
      <label class="g-field">
        <span>Import a file</span>
        <input type="file" accept="application/json,.json" data-testid="import-file" @change="upload" />
      </label>
    </div>
    <p v-if="problem" role="alert" class="g-alert" data-testid="transfer-problem">{{ problem }}</p>
    <div v-if="report" data-testid="import-report">
      <p>Imported {{ report.entries.length }} entries and {{ report.collections.length }} Collections.</p>
      <template v-if="report.manual.length">
        <p>To redo by hand:</p>
        <ul class="g-list">
          <li v-for="(m, i) in report.manual" :key="i" data-testid="manual">{{ m.where }}: {{ m.reason }}</li>
        </ul>
      </template>
    </div>
  </section>
</template>

<style scoped>
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 12px;
}
input[type='file'] {
  max-width: 100%;
}
</style>
