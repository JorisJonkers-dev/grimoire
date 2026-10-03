<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { watch } from 'vue'
import { getSessionLogOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LiveView, SessionAction } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = withDefaults(defineProps<{ campaignId: string; sessionId: string; view: LiveView | null; noUndo?: boolean }>(), { noUndo: false })
const emit = defineEmits<{ undo: [seq: number] }>()
const log = useQuery({ ...getSessionLogOptions({ path: { campaignId: props.campaignId, sessionId: props.sessionId }, query: { limit: 15 } }), retry: false })
// Every change to the table may add to the log.
watch(
  () => props.view,
  () => void log.refetch(),
)
const line = (a: SessionAction) => [a.kind.replaceAll('_', ' '), a.label].filter(Boolean).join(': ')
const who = (a: SessionAction) => (a.client ? `${a.actor} via ${a.client}` : a.actor)
</script>

<template>
  <section class="g-card log" aria-label="Action Log" data-testid="action-log">
    <h2>Action Log</h2>
    <p v-if="log.isError.value" role="alert" class="g-alert">The Action Log could not be read.</p>
    <ol v-else class="g-list">
      <li v-for="a in log.data.value ?? []" :key="a.seq" class="row" :data-testid="`log-${String(a.seq)}`">
        <span>
          <strong>#{{ a.seq }}</strong> {{ line(a) }} · {{ who(a) }}
        </span>
        <GButton v-if="a.undoable && !noUndo" :aria-label="`Undo #${String(a.seq)}: ${line(a)}`" :data-testid="`undo-${String(a.seq)}`" @click="emit('undo', a.seq)">Undo</GButton>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.log {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
</style>
