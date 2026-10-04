<script setup lang="ts">
// The parts of the Admin area: three are tabs of one page, two are pages of their own.
defineProps<{ current: 'accounts' | 'invites' | 'notes' | 'shared' | 'dice'; accounts?: number; invites?: number }>()
</script>

<template>
  <nav class="g-tabsnav" aria-label="Admin">
    <RouterLink :to="{ name: 'admin' }" :aria-current="current === 'accounts' ? 'page' : undefined">
      Accounts<span v-if="accounts !== undefined" class="count">{{ accounts }}</span>
    </RouterLink>
    <RouterLink :to="{ name: 'admin', query: { tab: 'invites' } }" :aria-current="current === 'invites' ? 'page' : undefined">
      Invites<span v-if="invites !== undefined" class="count">{{ invites }}</span>
    </RouterLink>
    <RouterLink :to="{ name: 'admin', query: { tab: 'notes' } }" :aria-current="current === 'notes' ? 'page' : undefined">Release Notes</RouterLink>
    <RouterLink :to="{ name: 'admin-shared' }" :aria-current="current === 'shared' ? 'page' : undefined" data-testid="admin-shared-link">Shared Library</RouterLink>
    <RouterLink :to="{ name: 'admin-dice' }" :aria-current="current === 'dice' ? 'page' : undefined" data-testid="admin-dice-link">Dice Sets</RouterLink>
  </nav>
</template>

<style scoped>
.count {
  font-family: var(--font-ui);
  font-size: 14px;
  color: var(--color-text-3);
}
</style>
