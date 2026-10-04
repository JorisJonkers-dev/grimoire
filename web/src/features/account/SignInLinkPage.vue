<script setup lang="ts">
import AuthShell from './AuthShell.vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { setAccountPasswordMutation, useSignInLinkMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Account } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'
import TwoStepForm from './TwoStepForm.vue'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const token = computed(() => route.hash.replace(/^#/, ''))
const valid = computed(() => /^[A-Za-z0-9_-]{20,64}$/.test(token.value))
const password = ref('')
const use = useMutation(useSignInLinkMutation())
const save = useMutation(setAccountPasswordMutation())
const account = ref<Account>()
const challenge = ref('')
function signedIn(a: Account) {
  account.value = a
  void client.invalidateQueries()
}
onMounted(() => {
  if (valid.value) use.mutate({ body: { token: token.value } }, { onSuccess: (out) => {
        if ('challenge' in out) challenge.value = out.challenge
        else signedIn(out)
      } })
})
function setPassword() {
  save.mutate({ body: { password: password.value } }, { onSuccess: () => void router.push({ name: 'home' }) })
}
</script>

<template>
  <AuthShell>
    <h1>Signing in</h1>
    <p v-if="!valid || use.isError.value" role="alert" class="g-alert" data-testid="link-invalid">
      This link has been used or has expired. Ask for a new one from the sign-in page.
    </p>
    <TwoStepForm v-else-if="challenge && !account" :challenge="challenge" @done="signedIn" @restart="router.push({ name: 'sign-in' })" />
    <p v-else-if="!account">Checking your link…</p>
    <form v-else class="stack" data-testid="new-password" @submit.prevent="setPassword">
      <p>Welcome back, {{ account.nickname }}. Choose a new password, or carry on without one.</p>
      <GField v-model="password" label="New password" type="password" :maxlength="200" autocomplete="new-password" data-testid="link-password" />
      <p v-if="save.isError.value" role="alert" class="g-alert">Use at least 10 characters.</p>
      <GButton type="submit" variant="primary" :disabled="password.length < 10 || save.isPending.value">Save the password</GButton>
      <RouterLink :to="{ name: 'home' }">Carry on</RouterLink>
    </form>
  </AuthShell>
</template>

<style scoped>
h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 34px;
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.stack p {
  margin: 0;
}
</style>
