<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import type { LivePrompt } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ prompt: LivePrompt; reactor: string; answerable: boolean }>()
const emit = defineEmits<{ answer: [use: boolean] }>()
const left = ref(props.prompt.secondsLeft)
let timer: ReturnType<typeof setInterval> | undefined
watch(
  () => props.prompt.id,
  () => {
    clearInterval(timer)
    left.value = props.prompt.secondsLeft
    timer = setInterval(() => {
      left.value = Math.max(0, left.value - 1)
    }, 1000)
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  clearInterval(timer)
})
const title = { opportunity_attack: 'Opportunity attack', shield: 'Shield', readied: 'Readied attack', effect: 'Reaction' }
</script>

<template>
  <section class="g-card prompt" role="alert" :aria-label="`${title[prompt.kind]} for ${reactor}`" data-testid="reaction-prompt">
    <header>
      <h2>{{ title[prompt.kind] }}: {{ reactor }}</h2>
      <span class="clock" data-testid="countdown">{{ left }} s</span>
    </header>
    <p data-testid="reaction-effect">{{ prompt.effect }}</p>
    <div v-if="answerable" class="row">
      <GButton variant="primary" aria-keyshortcuts="Y" data-testid="use-reaction" @click="emit('answer', true)">Use reaction</GButton>
      <GButton aria-keyshortcuts="N" aria-label="Decline the reaction" data-testid="decline-reaction" @click="emit('answer', false)">Decline</GButton>
    </div>
    <p v-else class="wait">Waiting for {{ reactor }}'s reaction.</p>
  </section>
</template>

<style scoped>
.prompt {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-color: var(--color-gold-high);
}
header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.clock {
  font-variant-numeric: tabular-nums;
  color: var(--color-gold-high);
}
p {
  margin: 0;
}
.wait {
  color: var(--color-text-2);
}
.row {
  display: flex;
  gap: 8px;
}
</style>
