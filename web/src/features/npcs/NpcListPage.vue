<script setup lang="ts">
import CampaignFrame from '@/features/campaigns/CampaignFrame.vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createNpcMutation, listDeletedNpcsOptions, listNpcsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Disposition } from '@/infrastructure/api/types.gen'
import { GAvatar, GButton, GField, GRow } from '@/shared/ui'

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
    <CampaignFrame current="npcs">
      <header class="g-headline">
        <span class="g-eyebrow">Campaign prep</span>
        <h1>NPCs</h1>
      </header>
    </CampaignFrame>
    <p v-if="npcs.isError.value" role="alert" class="g-alert" data-testid="npcs-refused">Only the DM can see the NPCs.</p>
    <template v-else>
      <ul class="g-list" data-testid="npc-list">
        <li v-for="n in npcs.data.value ?? []" :key="n.id">
          <GRow :to="{ name: 'npc', params: { id, npcId: n.id } }" :title="n.name" :subtitle="n.title || 'No title'">
            <template #leading><GAvatar :name="n.name" /></template>
            <template #trailing><span class="g-tag">{{ n.disposition }}</span></template>
          </GRow>
        </li>
      </ul>
      <form class="g-card add" data-testid="npc-create" @submit.prevent="add">
        <GField v-model="name" label="Name" :maxlength="80" data-testid="npc-name" />
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
.g-list {
  gap: 0;
}
.add {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h2 {
  margin: 0 0 8px;
}
.g-list a {
  color: var(--color-gold-high);
}
</style>
