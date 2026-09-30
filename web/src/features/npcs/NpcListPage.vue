<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createNpcMutation, listDeletedNpcsOptions, listNpcsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Disposition } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const id = computed(() => String(route.params.id))
const path = computed(() => ({ path: { campaignId: id.value } }))
const npcs = useQuery(computed(() => ({ ...listNpcsOptions(path.value), retry: false })))
const deleted = useQuery(computed(() => ({ ...listDeletedNpcsOptions(path.value), enabled: npcs.isSuccess.value })))
const create = useMutation(createNpcMutation())
const name = ref('')
const disposition = ref<Disposition>('neutral')

function add() {
  create.mutate(
    { ...path.value, body: { name: name.value.trim(), disposition: disposition.value } },
    {
      onSuccess: (n) => {
        name.value = ''
        void client.invalidateQueries()
        void router.push({ name: 'npc', params: { id: id.value, npcId: n.id } })
      },
    },
  )
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id } }" class="back">← Campaign</RouterLink>
    <h1>NPCs</h1>
    <p v-if="npcs.isError.value" role="alert" class="g-alert" data-testid="npcs-refused">Only the DM can see the NPCs.</p>
    <template v-else>
      <ul class="g-list" data-testid="npc-list">
        <li v-for="n in npcs.data.value ?? []" :key="n.id">
          <RouterLink :to="{ name: 'npc', params: { id, npcId: n.id } }" class="row">
            <span class="name">{{ n.name }}</span>
            <span class="meta">{{ n.title || 'No title' }}</span>
            <span class="g-tag">{{ n.disposition }}</span>
          </RouterLink>
        </li>
      </ul>
      <form class="g-card add" data-testid="npc-create" @submit.prevent="add">
        <label class="g-field"><span>Name</span><input v-model="name" maxlength="80" data-testid="npc-name" /></label>
        <label class="g-field">
          <span>Disposition</span>
          <select v-model="disposition">
            <option value="friendly">Friendly</option>
            <option value="neutral">Neutral</option>
            <option value="hostile">Hostile</option>
          </select>
        </label>
        <GButton type="submit" variant="primary" :disabled="name.trim() === '' || create.isPending.value">Add NPC</GButton>
      </form>
      <section v-if="(deleted.data.value ?? []).length" class="g-card" data-testid="npc-deleted">
        <h2>Recently deleted</h2>
        <ul class="g-list">
          <li v-for="n in deleted.data.value" :key="n.id">
            <RouterLink :to="{ name: 'npc', params: { id, npcId: n.id } }">{{ n.name }}</RouterLink>
          </li>
        </ul>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 2px 12px;
  min-height: 44px;
  padding: 10px 14px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
  text-decoration: none;
}
.name {
  font-weight: 700;
}
.meta {
  font-size: 14px;
  color: var(--color-text-2);
}
.row .g-tag {
  grid-row: 1 / span 2;
  grid-column: 2;
  align-self: center;
}
.add {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h2 {
  margin: 0 0 8px;
  font-family: var(--font-display);
  font-size: 17px;
}
.g-list a {
  color: var(--color-gold-high);
}
</style>
