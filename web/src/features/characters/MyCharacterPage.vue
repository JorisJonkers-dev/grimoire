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
import { GAvatar, GButton, GField } from '@/shared/ui'

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
    <nav aria-label="Breadcrumb" class="g-crumbs">
      <RouterLink :to="{ name: 'my-characters' }">Characters</RouterLink> <span aria-hidden="true">›</span>
      <span aria-current="page">{{ character.data.value?.name ?? 'Character' }}</span>
    </nav>
    <p v-if="character.isError.value" role="alert" class="g-alert" data-testid="my-character-error">This Character could not be read.</p>
    <template v-else-if="character.data.value">
      <header class="g-detail-head">
        <div class="who">
          <GAvatar :name="character.data.value.name" :size="96" aria-hidden="true" />
          <div class="names">
            <h1>{{ character.data.value.name }}</h1>
            <p class="g-meta">
              <span>{{ character.data.value.species }} {{ character.data.value.class }}</span>
              <span>{{ character.data.value.background }} background</span>
              <span>{{ character.data.value.ruleset }}</span>
            </p>
          </div>
        </div>
      </header>
      <div class="cols">
        <div class="main">
          <section data-testid="my-character-campaigns">
            <h2>In Campaigns</h2>
            <table v-if="character.data.value.campaigns.length">
              <tbody>
                <tr v-for="e in character.data.value.campaigns" :key="e.characterId">
                  <td><RouterLink :to="{ name: 'campaign', params: { id: e.campaignId } }" class="camp">{{ e.campaignName }}</RouterLink></td>
                  <td>Level {{ e.level }}</td>
                  <td class="dim">{{ e.hpCurrent }}/{{ e.hpMax }} HP</td>
                  <td class="open"><RouterLink :to="{ name: 'character', params: { id: e.campaignId, characterId: e.characterId } }" class="g-action">Sheet</RouterLink></td>
                </tr>
              </tbody>
            </table>
            <p v-else class="dim">Not in a Campaign yet.</p>
          </section>
          <form class="stack" data-testid="my-character-form" @submit.prevent="update">
            <h2>Identity</h2>
            <GField v-model="name" label="Name" :maxlength="60" required data-testid="my-character-name" />
            <GField v-model="backstory" label="Backstory" multiline :maxlength="4000" data-testid="my-character-backstory" />
            <p class="dim">Your DMs can read your Backstory to weave it into the story.</p>
            <p v-if="save.isSuccess.value" role="status" data-testid="my-character-saved">Saved in every Campaign.</p>
            <p v-if="save.isError.value" role="alert" class="g-alert">Give the Character a name of up to 60 characters.</p>
            <GButton type="submit" variant="primary" :disabled="!name.trim() || save.isPending.value">Save</GButton>
          </form>
        </div>
        <aside class="side">
          <h2>Another Campaign</h2>
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
          <p v-else class="dim">There is no other Campaign of yours to bring this Character into.</p>
        </aside>
      </div>
    </template>
  </main>
</template>

<style scoped>
.g-detail-head > .who {
  flex-direction: row;
  align-items: center;
  gap: 24px;
}
.names {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 36px 72px;
  align-items: start;
}
.main {
  display: flex;
  flex-direction: column;
  gap: 36px;
  min-width: 0;
}
.side,
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
h2 {
  margin: 0;
}
section h2 {
  margin-bottom: 10px;
}
p {
  margin: 0;
}
.camp {
  font-family: var(--font-display);
  font-size: 16px;
  font-weight: 600;
  text-decoration: none;
  color: var(--color-text);
}
.camp:hover {
  color: var(--color-gold-high);
}
td {
  padding: 13px 12px;
}
td:first-child {
  padding-left: 0;
}
.open {
  padding-right: 0;
  text-align: right;
}
.dim {
  font-size: 14px;
  color: var(--color-text-3);
}
td.dim {
  font-size: inherit;
  color: var(--color-text-2);
}
@media (max-width: 899px) {
  .cols {
    grid-template-columns: minmax(0, 1fr);
  }
  .g-detail-head > .who {
    gap: 14px;
  }
}
</style>
