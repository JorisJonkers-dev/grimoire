<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import NotificationBell from '@/features/notifications/NotificationBell.vue'
import { getAccountOptions, signOutMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { useOnline } from '@/shared/pwa/online'

const route = useRoute()
const router = useRouter()
const online = useOnline()
const bare = computed(() => route.meta.bare === true)
// A live page belongs to one Session: going to another Session, as a split party does, makes it anew.
const pageKey = computed(() => String(route.params.sid ?? ''))
const account = useQuery({ ...getAccountOptions(), retry: false })
const client = useQueryClient()
const signOut = useMutation(signOutMutation())
// The header's search box hands what was typed to the search page.
const sought = ref('')
function seek() {
  const q = sought.value.trim()
  sought.value = ''
  void router.push({ name: 'search', query: q ? { q } : {} })
}
function leave() {
  signOut.mutate({}, { onSuccess: () => { client.clear(); void router.push({ name: 'sign-in' }) } })
}
</script>

<template>
  <RouterView v-if="bare" :key="pageKey" />
  <div v-else class="shell">
    <header class="bar">
      <RouterLink :to="{ name: 'home' }" class="brand">Grimoire</RouterLink>
      <nav aria-label="Main">
        <RouterLink :to="{ name: 'campaigns' }">Campaigns</RouterLink>
        <RouterLink v-if="account.data.value" :to="{ name: 'my-characters' }" data-testid="characters-link">Characters</RouterLink>
        <RouterLink v-if="account.data.value" :to="{ name: 'library' }" data-testid="library-link">Library</RouterLink>
        <RouterLink v-if="account.data.value" :to="{ name: 'friends' }" data-testid="friends-link">Friends</RouterLink>
        <RouterLink v-if="account.data.value" :to="{ name: 'conversations' }" data-testid="conversations-link">Talk</RouterLink>
        <RouterLink v-if="account.data.value" :to="{ name: 'dice-sets' }" data-testid="dice-sets-link">Dice</RouterLink>
        <RouterLink :to="{ name: 'spells' }">Compendium</RouterLink>
        <RouterLink v-if="account.data.value?.adminPowers" :to="{ name: 'admin' }" data-testid="admin-link">Admin</RouterLink>
      </nav>
      <form v-if="route.name !== 'search'" role="search" aria-label="Search everything" class="seek" data-testid="header-search" @submit.prevent="seek">
        <input v-model="sought" type="search" maxlength="80" placeholder="Search" aria-label="Search everything" data-testid="header-search-input" />
      </form>
      <div class="me" data-testid="account-menu">
        <template v-if="account.data.value">
          <NotificationBell />
          <RouterLink :to="{ name: 'account' }" data-testid="account-link">{{ account.data.value.nickname }}</RouterLink>
          <button type="button" class="out" data-testid="sign-out" @click="leave">Sign out</button>
        </template>
        <RouterLink v-else-if="route.name !== 'sign-in'" :to="{ name: 'sign-in' }" data-testid="sign-in-link">Sign in</RouterLink>
      </div>
    </header>
    <p v-if="!online" role="status" class="offline" data-testid="offline">
      You are offline. The compendium and your Character sheets still open from this device; live play picks up again when the connection returns.
    </p>
    <RouterView :key="pageKey" />
    <footer class="credit">
      Grimoire by <a href="https://jorisjonkers.dev">Joris Jonkers</a> ·
      <RouterLink :to="{ name: 'attribution' }">SRD content under CC-BY-4.0</RouterLink> ·
      <RouterLink :to="{ name: 'automation' }">Automation coverage</RouterLink>
    </footer>
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.bar {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 0 var(--gutter);
  min-height: 56px;
  border-bottom: 1px solid var(--color-line);
  background: var(--color-surface);
}
.seek {
  margin-left: auto;
}
.seek input {
  width: 150px;
  max-width: 34vw;
  padding: 6px 10px;
  border: 1px solid var(--color-line);
  border-radius: 6px;
  background: var(--color-bg);
  color: var(--color-text);
}
.seek + .me {
  margin-left: 0;
}
.me {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}
.out {
  border: 0;
  background: none;
  color: var(--color-text-3);
  cursor: pointer;
}
.offline {
  margin: 0;
  padding: 8px var(--gutter);
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-gold);
  color: var(--color-gold-high);
}
.brand {
  font-family: var(--font-display);
  font-weight: 700;
  letter-spacing: 0.2em;
  color: var(--color-gold);
  text-decoration: none;
}
nav {
  display: flex;
  gap: 16px;
}
nav a {
  display: inline-flex;
  align-items: center;
  min-height: 54px;
  border-bottom: 2px solid transparent;
  color: var(--color-text);
  text-decoration: none;
}
nav a.router-link-active {
  border-bottom-color: var(--color-gold);
  color: var(--color-gold-high);
}
/* On a phone the links take their own row under the brand and the account, and scroll sideways. */
@media (max-width: 640px) {
  .bar {
    flex-wrap: wrap;
    gap: 0 16px;
  }
  nav {
    order: 3;
    width: 100%;
    overflow-x: auto;
  }
  nav a {
    min-height: 44px;
    white-space: nowrap;
  }
}
/* The footer spans the whole window, whatever the page above it does. */
.credit {
  margin-top: auto;
  width: 100%;
  box-sizing: border-box;
  padding: 16px var(--gutter);
  font-size: 14px;
  color: var(--color-text-2);
  border-top: 1px solid var(--color-line);
}
.credit a {
  color: var(--color-gold-high);
}
</style>
