<script setup lang="ts">
import { useInfiniteQuery, useMutation } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createCampaignMutation, listCampaignsInfiniteOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { CampaignPage } from '@/infrastructure/api/types.gen'
import { GAvatar, GButton, GField, GRow } from '@/shared/ui'

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
const create = useMutation(createCampaignMutation())
const canCreate = computed(() => name.value.trim() !== '' && displayName.value.trim() !== '' && !create.isPending.value)
function submit() {
  create.mutate(
    { body: { name: name.value.trim(), displayName: displayName.value.trim() } },
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
          <GRow :to="{ name: 'campaign', params: { id: c.id } }" :title="c.name" :subtitle="`${c.memberCount} ${c.memberCount === 1 ? 'member' : 'members'}`">
            <template #leading><GAvatar :name="c.name" /></template>
            <template #trailing><span class="g-tag">{{ c.myRole === 'dm' ? 'DM' : 'Player' }}</span></template>
          </GRow>
        </li>
      </ul>
      <GButton v-if="campaigns.hasNextPage.value" :disabled="campaigns.isFetchingNextPage.value" @click="campaigns.fetchNextPage()">
        Load more
      </GButton>
    </template>

    <form class="g-card create" data-testid="campaign-create" @submit.prevent="submit">
      <h2>Start a campaign</h2>
      <GField v-model="name" label="Campaign name" :maxlength="80" required data-testid="campaign-name" />
      <GField v-model="displayName" label="Your name at this table" :maxlength="60" required data-testid="campaign-display-name" />
      <p v-if="create.isError.value" role="alert" class="g-alert">The campaign could not be created. Check the names and try again.</p>
      <GButton type="submit" variant="primary" :disabled="!canCreate">Start as DM</GButton>
    </form>
  </main>
</template>

<style scoped>
.g-list {
  gap: 0;
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
