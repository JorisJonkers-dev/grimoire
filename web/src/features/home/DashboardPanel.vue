<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { getDashboardOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { DashboardNeed } from '@/infrastructure/api/types.gen'

// What a signed-in Member sees first: the Sessions under way, each one click away, and what needs
// them before the next one. Signed out there is nothing to show.
const dashboard = useQuery({ ...getDashboardOptions(), retry: false })
const live = computed(() => dashboard.data.value?.live ?? [])
const needs = computed(() => dashboard.data.value?.needs ?? [])
// What each kind of need is called, and the word on the link that deals with it.
const kinds: Record<DashboardNeed['kind'], { name: string; act: string }> = {
  level_up: { name: 'Advancement', act: 'Level up' },
  proposals: { name: 'Proposal', act: 'Review' },
  revise_proposal: { name: 'Proposal', act: 'Edit' },
  rolls: { name: 'Roll', act: 'Roll' },
  downtime: { name: 'Downtime', act: 'Spend' },
  friend_requests: { name: 'Friends', act: 'Answer' },
}
</script>

<template>
  <div v-if="dashboard.isSuccess.value" class="dashboard">
    <section v-if="live.length > 0" class="live" aria-label="Live now" data-testid="live-sessions">
      <div v-for="s in live" :key="s.sessionId" class="playing" data-testid="live-session">
        <span class="dot" aria-hidden="true" />
        <span class="what"><b>{{ s.campaign }}</b> is playing now <span class="dim">· Session {{ s.number }}<template v-if="s.dm"> · you are the DM</template></span></span>
        <RouterLink :to="{ name: 'session', params: { id: s.campaignId, sid: s.sessionId } }" class="join" :aria-label="`Join Session ${String(s.number)} of ${s.campaign}`">Join</RouterLink>
      </div>
    </section>
    <section class="needs" aria-labelledby="needs-title">
      <h2 id="needs-title">Needs you</h2>
      <p v-if="needs.length === 0" class="none" data-testid="nothing-needed">Nothing needs you before the next Session.</p>
      <div v-for="(n, i) in needs" :key="i" class="need" data-testid="need">
        <span class="title">{{ n.title }}</span>
        <RouterLink :to="n.path" class="g-action" :aria-label="`${kinds[n.kind].act}: ${n.title}`">{{ kinds[n.kind].act }}</RouterLink>
        <span class="about">{{ n.campaign ? `${n.campaign} · ` : '' }}{{ kinds[n.kind].name }}</span>
      </div>
    </section>
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 52px;
}
.live {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.playing {
  display: flex;
  align-items: center;
  gap: 18px;
  font-size: 18px;
}
.dot {
  width: 8px;
  height: 8px;
  flex: none;
  background: var(--color-success);
}
.what {
  flex: 1;
  min-width: 0;
}
.dim {
  color: var(--color-text-3);
}
.join {
  display: inline-flex;
  align-items: center;
  min-height: var(--size-control);
  padding: 0 20px;
  border: 1px solid var(--color-brass-edge);
  border-radius: var(--radius-control);
  background: var(--color-brass);
  color: var(--color-brass-text);
  font-family: var(--font-label);
  font-size: 16px;
  text-decoration: none;
}
.needs {
  display: flex;
  flex-direction: column;
}
h2 {
  margin: 0 0 6px;
}
.need {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 4px 24px;
  padding: 18px 0;
  border-bottom: 1px solid var(--color-rule-soft);
}
.need:last-child {
  border-bottom: 0;
}
.title {
  font-size: 17px;
}
.about,
.none {
  font-size: 14px;
  color: var(--color-text-3);
}
.none {
  margin: 12px 0 0;
  font-size: 16px;
}
@media (max-width: 899px) {
  .dashboard {
    gap: 30px;
  }
  .playing {
    flex-wrap: wrap;
    gap: 10px;
    font-size: 16px;
  }
  .need {
    padding: 14px 0;
  }
  .title {
    font-size: 16px;
  }
  .about {
    font-size: 13px;
  }
}
</style>
