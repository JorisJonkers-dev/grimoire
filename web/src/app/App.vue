<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getAccountOptions, signOutMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { useOnline } from '@/shared/pwa/online'

const route = useRoute()
const router = useRouter()
const online = useOnline()
const bare = computed(() => route.meta.bare === true)
const account = useQuery({ ...getAccountOptions(), retry: false })
const client = useQueryClient()
const signOut = useMutation(signOutMutation())
function leave() {
  signOut.mutate({}, { onSuccess: () => { client.clear(); void router.push({ name: 'sign-in' }) } })
}
</script>

<template>
  <RouterView v-if="bare" />
  <div v-else class="shell">
    <header class="bar">
      <RouterLink :to="{ name: 'home' }" class="brand">Grimoire</RouterLink>
      <nav aria-label="Main">
        <RouterLink :to="{ name: 'campaigns' }">Campaigns</RouterLink>
        <RouterLink v-if="account.data.value" :to="{ name: 'my-characters' }" data-testid="characters-link">Characters</RouterLink>
        <RouterLink :to="{ name: 'spells' }">Compendium</RouterLink>
        <RouterLink v-if="account.data.value?.adminPowers" :to="{ name: 'admin' }" data-testid="admin-link">Admin</RouterLink>
      </nav>
      <div class="me" data-testid="account-menu">
        <template v-if="account.data.value">
          <RouterLink :to="{ name: 'account' }" data-testid="account-link">{{ account.data.value.nickname }}</RouterLink>
          <button type="button" class="out" data-testid="sign-out" @click="leave">Sign out</button>
        </template>
        <RouterLink v-else-if="route.name !== 'sign-in'" :to="{ name: 'sign-in' }" data-testid="sign-in-link">Sign in</RouterLink>
      </div>
    </header>
    <p v-if="!online" role="status" class="offline" data-testid="offline">
      You are offline. The compendium and your Character sheets still open from this device; live play picks up again when the connection returns.
    </p>
    <RouterView />
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
