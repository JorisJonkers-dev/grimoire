<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getMyCharacterOptions,
  getMyCharacterQueryKey,
  joinCampaignMutation,
  listCampaignsOptions,
  updateMyCharacterMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const path = computed(() => ({ characterId: String(route.params.characterId) }))
const character = useQuery(computed(() => getMyCharacterOptions({ path: path.value })))
const campaigns = useQuery(listCampaignsOptions())
const save = useMutation(updateMyCharacterMutation())
const join = useMutation(joinCampaignMutation())
const name = ref('')
const backstory = ref('')
const target = ref('')
watch(character.data, (c) => {
  if (!c) return
  name.value = c.name
  backstory.value = c.backstory
}, { immediate: true })
const open = computed(() => {
  const playing = new Set(character.data.value?.campaigns.map((e) => e.campaignId))
  return (campaigns.data.value?.items ?? []).filter((c) => !playing.has(c.id))
})
const joinFailed = computed(() => {
  const status = (join.error.value as { status?: number } | null)?.status
  if (status === 422) return 'That Campaign\'s rules do not allow this Character\'s build.'
  return status === 409 ? 'This Character already plays there.' : 'The Character could not join.'
})
function update() {
  save.mutate({ path: path.value, body: { name: name.value.trim(), backstory: backstory.value } }, { onSuccess: () => void client.invalidateQueries({ queryKey: getMyCharacterQueryKey({ path: path.value }) }) })
}
function bring() {
  join.mutate({ path: path.value, body: { campaignId: target.value } }, { onSuccess: (sheet) => {
      void client.invalidateQueries()
      void router.push({ name: 'character', params: { id: target.value, characterId: sheet.id } })
    } })
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'my-characters' }">All your Characters</RouterLink>
    <p v-if="character.isError.value" role="alert" class="g-alert" data-testid="my-character-error">This Character could not be read.</p>
    <template v-else-if="character.data.value">
      <h1>{{ character.data.value.name }}</h1>
      <p class="dim">{{ character.data.value.species }} {{ character.data.value.class }} · {{ character.data.value.background }} · {{ character.data.value.ruleset }}</p>
      <form class="g-card stack" data-testid="my-character-form" @submit.prevent="update">
        <h2>Identity</h2>
        <GField v-model="name" label="Name" :maxlength="60" required data-testid="my-character-name" />
        <GField v-model="backstory" label="Backstory" multiline :maxlength="4000" data-testid="my-character-backstory" />
        <p v-if="save.isSuccess.value" role="status" data-testid="my-character-saved">Saved in every Campaign.</p>
        <p v-if="save.isError.value" role="alert" class="g-alert">Give the Character a name of up to 60 characters.</p>
        <GButton type="submit" :disabled="!name.trim() || save.isPending.value">Save</GButton>
      </form>
      <section class="g-card stack" data-testid="my-character-campaigns">
        <h2>Campaigns</h2>
        <ul v-if="character.data.value.campaigns.length" class="rows">
          <li v-for="e in character.data.value.campaigns" :key="e.characterId">
            <RouterLink :to="{ name: 'character', params: { id: e.campaignId, characterId: e.characterId } }">{{ e.campaignName }}</RouterLink>
            <span class="dim">level {{ e.level }} · {{ e.hpCurrent }}/{{ e.hpMax }} HP</span>
          </li>
        </ul>
        <p v-else>Not in a Campaign yet.</p>
        <form v-if="open.length" class="stack" data-testid="my-character-join" @submit.prevent="bring">
          <label class="g-field">
            <span>Bring into</span>
            <select v-model="target" data-testid="my-character-target">
              <option value="" disabled>Choose a Campaign</option>
              <option v-for="c in open" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </label>
          <p class="dim">The new Campaign gets its own progress, from first level.</p>
          <p v-if="join.isError.value" role="alert" class="g-alert" data-testid="my-character-join-failed">{{ joinFailed }}</p>
          <GButton type="submit" :disabled="!target || join.isPending.value">Join the Campaign</GButton>
        </form>
      </section>
    </template>
  </main>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
h2 {
  margin: 0;
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.rows li {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
}
.dim {
  color: var(--color-text-3);
  font-size: 14px;
}
</style>
