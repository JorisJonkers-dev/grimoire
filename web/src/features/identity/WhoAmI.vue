<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { getMeOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

const me = useQuery({ ...getMeOptions(), retry: false })
</script>

<template>
  <p class="who" data-testid="who-am-i">
    <template v-if="me.data.value">Signed in as <strong>{{ me.data.value.subject }}</strong></template>
    <template v-else-if="me.isError.value">You are not signed in.</template>
    <template v-else>Checking who you are…</template>
  </p>
</template>

<style scoped>
.who {
  margin: 0;
  color: var(--color-text-2);
}
strong {
  color: var(--color-text);
}
</style>
