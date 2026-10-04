<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getCampaignOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

// What every page of a Campaign shares: where it is, then (after the page's own name) the Campaign's pages as tabs.
const props = withDefaults(defineProps<{ current: string; crumbs?: boolean }>(), { crumbs: true })
const route = useRoute()
const id = computed(() => String(route.params.id))
const campaign = useQuery(computed(() => ({ ...getCampaignOptions({ path: { campaignId: id.value } }), retry: false })))
const isDM = computed(() => campaign.data.value?.myRole === 'dm')
const pages = [
  { name: 'campaign', label: 'Overview' },
  { name: 'journal', label: 'Journal' },
  { name: 'maps', label: 'Maps', dm: true },
  { name: 'npcs', label: 'NPCs', dm: true },
  { name: 'factions', label: 'Factions' },
  { name: 'encounters', label: 'Encounters', dm: true },
  { name: 'loot', label: 'Loot', dm: true },
  { name: 'shops', label: 'Shops', dm: true },
  { name: 'campaign-library', label: 'Library', dm: true, testid: 'library' },
  { name: 'proposals', label: 'Proposals' },
  { name: 'tracks', label: 'Tracks' },
  { name: 'downtime', label: 'Downtime' },
  { name: 'vehicles', label: 'Vehicles' },
  { name: 'rule-variants', label: 'Rule Variants', testid: 'rules' },
  { name: 'dice', label: 'Dice' },
  { name: 'activity', label: 'AI activity', dm: true },
]
const shown = computed(() => pages.filter((p) => !p.dm || isDM.value))
const here = computed(() => pages.find((p) => p.name === props.current)?.label ?? '')
</script>

<template>
  <nav v-if="crumbs" aria-label="Breadcrumb" class="g-crumbs">
    <RouterLink :to="{ name: 'campaigns' }">Campaigns</RouterLink> <span aria-hidden="true">›</span>
    <RouterLink :to="{ name: 'campaign', params: { id } }" data-testid="to-campaign">{{ campaign.data.value?.name ?? 'Campaign' }}</RouterLink> <span aria-hidden="true">›</span>
    <span aria-current="page">{{ here }}</span>
  </nav>
  <slot />
  <nav class="g-tabsnav" aria-label="Campaign">
    <RouterLink
      v-for="p in shown"
      :key="p.name"
      :to="{ name: p.name, params: { id } }"
      :aria-current="p.name === current ? 'page' : undefined"
      :data-testid="`${p.testid ?? p.name}-link`"
    >
      {{ p.label }}
    </RouterLink>
  </nav>
</template>
