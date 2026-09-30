<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useLiveSession, type SocketFactory } from '@/realtime/liveSession'
import HexGrid from '@/shared/map/HexGrid.vue'
import { board } from './board'

const props = defineProps<{ socket?: SocketFactory }>()
const route = useRoute()
const { view } = useLiveSession(String(route.params.id), String(route.params.sid), 'table', props.socket)
const cells = computed(() => board(view.session?.gridRadius ?? 0, view.tokens, null))
</script>

<template>
  <main class="table" data-testid="table-display">
    <h1 class="sr-only">Table display</h1>
    <p v-if="view.connection === 'ended'" class="ended" role="status">The session has ended.</p>
    <p v-else-if="!view.session" class="ended" role="status">Waiting for the table…</p>
    <HexGrid v-else :cells="cells" :size="36" title="The table" />
  </main>
</template>

<style scoped>
.table {
  display: grid;
  place-items: center;
  min-height: 100vh;
  background: var(--color-ground);
  overflow: auto;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.ended {
  font-family: var(--font-display);
  font-size: 28px;
  color: var(--color-text-2);
}
</style>
