<script setup lang="ts">
import { useInfiniteQuery, useMutation } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createCampaignMutation, listCampaignsInfiniteOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { CampaignPage, Ruleset } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const router = useRouter()
const campaigns = useInfiniteQuery(
  computed(() => ({
    ...listCampaignsInfiniteOptions(),
    initialPageParam: {},
    getNextPageParam: (last: CampaignPage) => last.nextCursor,
  })),
)
const items = computed(() => campaigns.data.value?.pages.flatMap((p) => p.items) ?? [])

const name = ref('')
const displayName = ref('')
const ruleset = ref<Ruleset>('srd-2024')
const create = useMutation(createCampaignMutation())
const canCreate = computed(() => name.value.trim() !== '' && displayName.value.trim() !== '' && !create.isPending.value)
function submit() {
  create.mutate(
    { body: { name: name.value.trim(), displayName: displayName.value.trim(), ruleset: ruleset.value } },
    { onSuccess: (campaign) => void router.push({ name: 'campaign', params: { id: campaign.id } }) },
  )
}
</script>

<template>
  <main class="g-page">
    <h1>Campaigns</h1>
    <p v-if="campaigns.isPending.value">Gathering your tables…</p>
    <p v-else-if="campaigns.isError.value" role="alert" class="g-alert">Your campaigns could not be loaded. Try again shortly.</p>
    <template v-else>
      <p v-if="items.length === 0" data-testid="campaign-empty">You are not in any campaign yet. Start one, or ask your DM for an invite link.</p>
      <ul class="g-list" data-testid="campaign-list">
        <li v-for="c in items" :key="c.id">
          <RouterLink :to="{ name: 'campaign', params: { id: c.id } }" class="row">
            <span class="name">{{ c.name }}</span>
            <span class="meta">{{ c.memberCount }} {{ c.memberCount === 1 ? 'member' : 'members' }} · {{ c.ruleset === 'srd-2024' ? '2024 rules' : '2014 rules' }}</span>
            <span class="g-tag role">{{ c.myRole === 'dm' ? 'DM' : 'Player' }}</span>
          </RouterLink>
        </li>
      </ul>
      <GButton v-if="campaigns.hasNextPage.value" :disabled="campaigns.isFetchingNextPage.value" @click="campaigns.fetchNextPage()">
        Load more
      </GButton>
    </template>

    <form class="g-card create" data-testid="campaign-create" @submit.prevent="submit">
      <h2>Start a campaign</h2>
      <label class="g-field">
        <span>Campaign name</span>
        <input v-model="name" maxlength="80" required data-testid="campaign-name" />
      </label>
      <label class="g-field">
        <span>Your name at this table</span>
        <input v-model="displayName" maxlength="60" required data-testid="campaign-display-name" />
      </label>
      <label class="g-field">
        <span>Rules</span>
        <select v-model="ruleset">
          <option value="srd-2024">2024 rules</option>
          <option value="srd-2014">2014 rules</option>
        </select>
      </label>
      <p v-if="create.isError.value" role="alert" class="g-alert">The campaign could not be created. Check the names and try again.</p>
      <GButton type="submit" variant="primary" :disabled="!canCreate">Start as DM</GButton>
    </form>
  </main>
</template>

<style scoped>
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
.row:hover,
.row:focus-visible {
  border-color: var(--color-gold);
}
.name {
  font-weight: 700;
}
.meta {
  font-size: 14px;
  color: var(--color-text-2);
}
.role {
  grid-row: 1 / span 2;
  grid-column: 2;
  align-self: center;
}
.create {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.create h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 18px;
}
</style>
