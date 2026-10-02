<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { setAccountPasswordMutation, useSignInLinkMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const token = computed(() => route.hash.replace(/^#/, ''))
const valid = computed(() => /^[A-Za-z0-9_-]{20,64}$/.test(token.value))
const password = ref('')
const use = useMutation(useSignInLinkMutation())
const save = useMutation(setAccountPasswordMutation())
onMounted(() => {
  if (valid.value) use.mutate({ body: { token: token.value } }, { onSuccess: () => void client.invalidateQueries() })
})
function setPassword() {
  save.mutate({ body: { password: password.value } }, { onSuccess: () => void router.push({ name: 'home' }) })
}
</script>

<template>
  <main class="g-page narrow">
    <h1>Signing in</h1>
    <p v-if="!valid || use.isError.value" role="alert" class="g-alert" data-testid="link-invalid">
      This link has been used or has expired. Ask for a new one from the sign-in page.
    </p>
    <p v-else-if="!use.data.value">Checking your link…</p>
    <form v-else class="g-card stack" data-testid="new-password" @submit.prevent="setPassword">
      <p>Welcome back, {{ use.data.value.nickname }}. Choose a new password, or carry on without one.</p>
      <GField v-model="password" label="New password" type="password" :maxlength="200" autocomplete="new-password" data-testid="link-password" />
      <p v-if="save.isError.value" role="alert" class="g-alert">Use at least 10 characters.</p>
      <GButton type="submit" variant="primary" :disabled="password.length < 10 || save.isPending.value">Save the password</GButton>
      <RouterLink :to="{ name: 'home' }">Carry on</RouterLink>
    </form>
  </main>
</template>

<style scoped>
.narrow {
  max-width: 420px;
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
