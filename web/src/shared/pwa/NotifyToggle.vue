<script setup lang="ts">
import { ref } from 'vue'
import { GButton } from '@/shared/ui'
import { pushSupported, subscribeDevice, type Subscribed } from './notifications'

const state = ref<Subscribed | 'idle' | 'failed'>('idle')
const said: Record<Exclude<typeof state.value, 'idle'>, string> = {
  subscribed: 'This device will hear about your turns.',
  denied: 'Notifications are blocked for Grimoire in this browser.',
  unavailable: 'This server sends no notifications.',
  failed: 'Notifications could not be turned on. Try again shortly.',
}
async function turnOn() {
  state.value = await subscribeDevice().catch(() => 'failed' as const)
}
</script>

<template>
  <div v-if="pushSupported()" class="notify" data-testid="notify">
    <GButton v-if="state === 'idle'" data-testid="notify-on" @click="turnOn()">Notify me on my turn</GButton>
    <p v-else role="status" data-testid="notify-state">{{ said[state] }}</p>
  </div>
</template>

<style scoped>
.notify p {
  margin: 0;
  color: var(--color-text-2);
  font-size: 14px;
}
</style>
