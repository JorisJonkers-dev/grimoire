<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import DashboardPanel from '@/features/home/DashboardPanel.vue'
import WhoAmI from '@/features/identity/WhoAmI.vue'
import ReleaseNoteCard from '@/features/releases/ReleaseNoteCard.vue'
import StatusPanel from '@/features/status/StatusPanel.vue'
import { getAccountOptions, getDashboardOptions, listCampaignsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

// The Dashboard: what needs me on the left, my Campaigns and what is new on the right. A visitor
// who is not signed in gets the brand and nothing of anybody's.
const dashboard = useQuery({ ...getDashboardOptions(), retry: false })
const account = useQuery({ ...getAccountOptions(), retry: false })
const signedIn = computed(() => dashboard.isSuccess.value)
const campaigns = useQuery(computed(() => ({ ...listCampaignsOptions(), enabled: signedIn.value, retry: false })))
const mine = computed(() => campaigns.data.value?.items ?? [])

const hour = new Date().getHours()
const part = hour < 12 ? 'morning' : hour < 18 ? 'afternoon' : 'evening'
const greeting = computed(() => `Good ${part}${account.data.value ? `, ${account.data.value.nickname}` : ''}.`)
const words = ['Nothing', 'One thing', 'Two things', 'Three things', 'Four things', 'Five things', 'Six things', 'Seven things', 'Eight things', 'Nine things']
const title = computed(() => {
  const n = dashboard.data.value?.needs.length ?? 0
  return `${words[n] ?? `${String(n)} things`} ${n <= 1 ? 'needs' : 'need'} you`
})
</script>

<template>
  <main v-if="signedIn" class="g-page home" data-testid="dashboard">
    <div class="main">
      <header class="head">
        <p class="g-flavour" data-testid="greeting">{{ greeting }}</p>
        <h1>{{ title }}</h1>
      </header>
      <DashboardPanel />
    </div>
    <aside class="side">
      <section class="block" aria-labelledby="dash-campaigns">
        <h2 id="dash-campaigns">Campaigns</h2>
        <RouterLink v-for="c in mine" :key="c.id" :to="{ name: 'campaign', params: { id: c.id } }" class="campaign" data-testid="dash-campaign">
          <span class="name">{{ c.name }}</span>
          <span class="role">{{ c.myRole === 'dm' ? 'Dungeon Master' : 'Player' }}</span>
        </RouterLink>
        <RouterLink :to="{ name: 'campaigns' }" class="g-action" data-testid="dash-new-campaign">{{ mine.length ? 'All Campaigns' : 'New Campaign' }}</RouterLink>
      </section>
      <ReleaseNoteCard />
      <StatusPanel />
      <WhoAmI />
    </aside>
  </main>
  <main v-else class="g-page visitor">
    <h1>Grimoire</h1>
    <p class="g-flavour">The table is set. Nothing stirs yet.</p>
    <ReleaseNoteCard />
    <DashboardPanel />
    <WhoAmI />
    <StatusPanel />
  </main>
</template>

<style scoped>
.home {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 260px;
  column-gap: 96px;
  align-items: start;
  padding-top: 56px;
}
.main {
  display: flex;
  flex-direction: column;
  gap: 52px;
}
.head {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.side {
  display: flex;
  flex-direction: column;
  gap: 44px;
  padding-top: 8px;
}
.block {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.campaign {
  display: flex;
  flex-direction: column;
  gap: 3px;
  color: var(--color-text);
  text-decoration: none;
}
.campaign:hover .name {
  color: var(--color-gold-high);
}
.name {
  font-family: var(--font-display);
  font-weight: 600;
  font-size: 17px;
}
.role {
  font-size: 14px;
  color: var(--color-text-3);
}
.block .g-action {
  align-self: flex-start;
  font-size: 15px;
  text-decoration: none;
}
.visitor h1 {
  font-size: 40px;
}
@media (max-width: 899px) {
  .home {
    grid-template-columns: minmax(0, 1fr);
    row-gap: 36px;
    padding-top: 28px;
  }
  .main {
    gap: 30px;
  }
  .side {
    gap: 30px;
  }
  .g-flavour {
    font-size: 16px;
  }
}
</style>
