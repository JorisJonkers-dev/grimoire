<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { acceptAccountInviteMutation, previewAccountInviteMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const token = computed(() => route.hash.replace(/^#/, ''))
const valid = computed(() => /^[A-Za-z0-9_-]{20,64}$/.test(token.value))
const form = ref({ username: '', nickname: '', email: '', password: '' })
const preview = useMutation(previewAccountInviteMutation())
const accept = useMutation(acceptAccountInviteMutation())
onMounted(() => {
  if (valid.value) preview.mutate({ body: { token: token.value } })
})
const usernameRule = (v: string): string | undefined => (/^[a-z0-9][a-z0-9_.-]{2,31}$/.test(v.toLowerCase()) ? undefined : 'Use 3 to 32 letters, digits, dots, dashes or underscores.')
const passwordRule = (v: string): string | undefined => (v.length >= 10 ? undefined : 'Use at least 10 characters.')
const ready = computed(() => !usernameRule(form.value.username.trim()) && form.value.nickname.trim() && form.value.email.includes('@') && !passwordRule(form.value.password))
function submit() {
  const f = form.value
  accept.mutate(
    { body: { token: token.value, username: f.username.trim(), nickname: f.nickname.trim(), email: f.email.trim(), password: f.password } },
    { onSuccess: () => {
        void client.invalidateQueries()
        void router.push({ name: 'home' })
      } },
  )
}
</script>

<template>
  <main class="g-page narrow">
    <h1>Set up your Account</h1>
    <p v-if="!valid || preview.isError.value" role="alert" class="g-alert" data-testid="account-invite-invalid">
      This invite has been used or has expired. Ask an Admin for a new one.
    </p>
    <p v-else-if="!preview.data.value">Reading the invite…</p>
    <form v-else class="g-card stack" data-testid="account-setup" @submit.prevent="submit">
      <p v-if="preview.data.value.admin">You are invited as an Admin.</p>
      <GField v-model="form.username" label="Username" :maxlength="32" autocomplete="username" required :rules="[usernameRule]" hint="You sign in with it." data-testid="setup-username" />
      <GField v-model="form.nickname" label="Nickname" :maxlength="40" required hint="What others see." data-testid="setup-nickname" />
      <GField v-model="form.email" label="Email" type="email" :maxlength="254" autocomplete="email" required data-testid="setup-email" />
      <GField v-model="form.password" label="Password" type="password" :maxlength="200" autocomplete="new-password" required :rules="[passwordRule]" data-testid="setup-password" />
      <p v-if="accept.isError.value" role="alert" class="g-alert" data-testid="setup-failed">That Username or email may be taken, or the invite has expired.</p>
      <GButton type="submit" variant="primary" :disabled="!ready || accept.isPending.value">Create my Account</GButton>
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
