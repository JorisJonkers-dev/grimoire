<script setup lang="ts">
import { useServiceStatus } from './useServiceStatus'

const status = useServiceStatus()
const formatDate = (iso: string) => new Date(iso).toLocaleDateString('en-GB', { dateStyle: 'medium' })
</script>

<template>
  <section class="panel" aria-labelledby="status-title" data-testid="status-panel">
    <h2 id="status-title">Service</h2>
    <p v-if="status.isPending.value" data-testid="status-loading">Checking the server…</p>
    <p v-else-if="status.isError.value" role="alert" data-testid="status-error">The server is not answering. Try again shortly.</p>
    <dl v-else-if="status.data.value" data-testid="status-ok">
      <dt>Version</dt>
      <dd>{{ status.data.value.version }}</dd>
      <dt>Database</dt>
      <dd>{{ status.data.value.database }}</dd>
      <dt>Running since</dt>
      <dd>{{ formatDate(status.data.value.startedAt) }}</dd>
    </dl>
  </section>
</template>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 14px;
  color: var(--color-text-2);
}
h2 {
  margin: 0;
}
p {
  margin: 0;
}
dl {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 16px;
  margin: 0;
}
dt {
  color: var(--color-text-3);
}
dd {
  margin: 0;
}
</style>
