<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { listActivityOptions, undoChangeMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Activity } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const activity = useQuery({ ...listActivityOptions({ path: { campaignId } }), retry: false })
const undo = useMutation(undoChangeMutation())
const failed = ref('')
const endpoint = `${window.location.origin}/mcp`
const kinds: Record<Activity['entityType'], string> = {
  npc: 'NPC', encounter_pool: 'encounter pool', encounter_table: 'encounter table', encounter_check: 'encounter check',
  loot_table: 'loot table', settlement: 'settlement', shop: 'shop',
}
const verbs: Record<Activity['action'], string> = { create: 'created', update: 'changed', delete: 'deleted', restore: 'restored' }
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
const line = (a: Activity) => `${a.client ?? 'An agent'} ${verbs[a.action]} the ${kinds[a.entityType]} ${a.name}`
function revert(a: Activity) {
  undo.mutate(
    { path: { campaignId, revisionId: a.revisionId } },
    {
      onSuccess: () => {
        failed.value = ''
        void client.invalidateQueries()
      },
      onError: (err) => {
        const detail = (err as { detail?: string } | null)?.detail
        failed.value = detail ? `Undo: ${detail}` : 'Undo failed. Try again shortly.'
      },
    },
  )
}
</script>

<template>
  <main class="g-page activity">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <h1>AI activity</h1>
    <p v-if="activity.isError.value" role="alert" class="g-alert" data-testid="activity-refused">Only the DM can see AI activity.</p>
    <template v-else>
      <p class="hint">
        Connect an AI agent to Grimoire's MCP server and it can prepare this campaign as you. Its changes apply at once and show up here; Undo puts
        things back as they were.
      </p>
      <label class="g-field"><span>MCP server</span><input :value="endpoint" readonly data-testid="mcp-url" /></label>
      <p v-if="failed" role="alert" class="g-alert" data-testid="activity-error">{{ failed }}</p>
      <p v-if="activity.data.value?.length === 0" data-testid="activity-empty">Nothing yet. Changes your agent makes will appear here.</p>
      <ul class="g-list">
        <li v-for="a in activity.data.value ?? []" :key="a.revisionId" class="row" :data-testid="`activity-${a.name}-${String(a.no)}`">
          <span>
            <strong>{{ line(a) }}</strong> · revision {{ a.no }} · {{ a.author }} · {{ when(a.createdAt) }}
            <span v-if="!a.undoable" class="g-tag">changed since</span>
          </span>
          <GButton v-if="a.undoable" :aria-label="`Undo: ${line(a)}`" :data-testid="`undo-${a.name}-${String(a.no)}`" @click="revert(a)">Undo</GButton>
        </li>
      </ul>
    </template>
  </main>
</template>

<style scoped>
.activity {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.back {
  color: var(--color-gold-high);
}
p {
  margin: 0;
}
.hint {
  color: var(--color-text-2);
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
</style>
