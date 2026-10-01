<script setup lang="ts">
import type { Revision } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

defineProps<{ revisions: Revision[]; label: string }>()
const emit = defineEmits<{ restore: [no: number] }>()
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
</script>

<template>
  <section class="history" :aria-label="`History of ${label}`">
    <h3>History</h3>
    <ul class="g-list">
      <li v-for="r in revisions" :key="r.no" class="row">
        <span>
          <strong>#{{ r.no }} {{ r.action }}</strong><template v-if="r.restoredFrom"> from #{{ r.restoredFrom }}</template> · {{ r.author }} ·
          {{ when(r.createdAt) }}
        </span>
        <GButton :aria-label="`Restore ${label} to revision ${String(r.no)}`" @click="emit('restore', r.no)">Restore</GButton>
      </li>
    </ul>
  </section>
</template>

<style scoped>
h3 {
  margin: 0;
  font-size: 16px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
</style>
