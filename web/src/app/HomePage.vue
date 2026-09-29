<script setup lang="ts">
import WhoAmI from '@/features/identity/WhoAmI.vue'
import StatusPanel from '@/features/status/StatusPanel.vue'
</script>

<template>
  <main class="home">
    <h1>Grimoire</h1>
    <p class="flavour">The table is set. Nothing stirs yet.</p>
    <WhoAmI />
    <StatusPanel />
  </main>
</template>

<style scoped>
.home {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 640px;
  margin: 0 auto;
  padding: 24px 16px;
}
h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 40px;
}
.flavour {
  margin: 0;
  font-family: var(--font-flavour);
  font-style: italic;
  color: var(--color-text-2);
}
</style>
