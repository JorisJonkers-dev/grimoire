<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useOnline } from '@/shared/pwa/online'

const route = useRoute()
const online = useOnline()
const bare = computed(() => route.meta.bare === true)
</script>

<template>
  <RouterView v-if="bare" />
  <div v-else class="shell">
    <header class="bar">
      <RouterLink :to="{ name: 'home' }" class="brand">Grimoire</RouterLink>
      <nav aria-label="Main">
        <RouterLink :to="{ name: 'campaigns' }">Campaigns</RouterLink>
        <RouterLink :to="{ name: 'spells' }">Compendium</RouterLink>
      </nav>
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
