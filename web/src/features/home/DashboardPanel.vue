<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { getDashboardOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

// What a signed-in Member sees first: the Sessions under way, each one click away, and what needs
// them before the next one. Signed out there is nothing to show.
const dashboard = useQuery({ ...getDashboardOptions(), retry: false })
const live = computed(() => dashboard.data.value?.live ?? [])
const needs = computed(() => dashboard.data.value?.needs ?? [])
</script>

<template>
  <div v-if="dashboard.isSuccess.value" class="dashboard" data-testid="dashboard">
    <section v-if="live.length > 0" class="g-card" aria-labelledby="live-title" data-testid="live-sessions">
      <h2 id="live-title">Under way now</h2>
      <ul class="g-list">
        <li v-for="s in live" :key="s.sessionId" class="row" data-testid="live-session">
          <span>{{ s.campaign }} · Session {{ s.number }}<template v-if="s.dm"> · you are the DM</template></span>
          <RouterLink
            :to="{ name: 'session', params: { id: s.campaignId, sid: s.sessionId } }"
            class="join"
            :aria-label="`Join Session ${String(s.number)} of ${s.campaign}`"
          >
            Join
          </RouterLink>
        </li>
      </ul>
    </section>
    <section class="g-card" aria-labelledby="needs-title">
      <h2 id="needs-title">Before the next Session</h2>
      <p v-if="needs.length === 0" class="hint" data-testid="nothing-needed">Nothing needs you before the next Session.</p>
      <ul v-else class="g-list">
        <li v-for="(n, i) in needs" :key="i" data-testid="need">
          <RouterLink :to="n.path">{{ n.title }}</RouterLink><template v-if="n.campaign"> · {{ n.campaign }}</template>
        </li>
      </ul>
    </section>
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
h2 {
  margin: 0 0 8px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.join {
  padding: 6px 14px;
  border: 1px solid var(--color-gold-high);
  border-radius: 6px;
  color: var(--color-gold-high);
  text-decoration: none;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
a {
  color: var(--color-gold-high);
}
</style>
