<script setup lang="ts">
import { useMutation } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { passTwoStepMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Account } from '@/infrastructure/api/types.gen'
import { GButton, GCodeField, GField } from '@/shared/ui'

const props = defineProps<{ challenge: string }>()
const emit = defineEmits<{ done: [account: Account]; restart: [] }>()
const code = ref('')
const recovery = ref(false)
const pass = useMutation(passTwoStepMutation())
const status = computed(() => (pass.error.value as { status?: number } | null)?.status)
const ready = computed(() => (recovery.value ? code.value.replace(/[\s-]/g, '').length === 10 : code.value.length === 6))
function submit() {
  pass.mutate({ body: { challenge: props.challenge, code: code.value.trim() } }, {
    onSuccess: (a) => { emit('done', a) },
    onError: () => { code.value = '' },
  })
}
function toggle() {
  recovery.value = !recovery.value
  code.value = ''
}
</script>

<template>
  <form class="g-card stack" data-testid="two-step-form" @submit.prevent="submit">
    <h2>Two-step sign-in</h2>
    <template v-if="!recovery">
      <p>Enter the six-digit code from your authenticator app.</p>
      <GCodeField v-model="code" data-testid="two-step-code" />
    </template>
    <template v-else>
      <p>Enter one of your recovery codes. Each works once.</p>
      <GField v-model="code" label="Recovery code" :maxlength="16" autocomplete="off" data-testid="two-step-recovery" />
    </template>
    <p v-if="status === 410" role="alert" class="g-alert" data-testid="two-step-expired">
      This sign-in expired or had too many wrong codes.
      <button type="button" class="link" @click="emit('restart')">Sign in again</button>
    </p>
    <p v-else-if="pass.isError.value" role="alert" class="g-alert" data-testid="two-step-wrong">That code is wrong or was already used.</p>
    <GButton type="submit" variant="primary" :disabled="!ready || pass.isPending.value">Continue</GButton>
    <button type="button" class="link" data-testid="two-step-toggle" @click="toggle">
      {{ recovery ? 'Use the authenticator app' : 'Use a recovery code instead' }}
    </button>
  </form>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
h2 {
  margin: 0;
}
.link {
  align-self: flex-start;
  padding: 0;
  border: 0;
  background: none;
  color: var(--color-gold);
  cursor: pointer;
  text-decoration: underline;
}
</style>
