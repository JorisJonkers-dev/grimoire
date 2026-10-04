<script setup lang="ts">
import AuthShell from './AuthShell.vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createOidcAccountMutation, finishOidcMutation, getSignInMethodsOptions, linkOidcAccountMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'
import { returnTo } from './leave'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const methods = useQuery(getSignInMethodsOptions())
const provider = computed(() => methods.data.value?.oidc ?? 'your external login')
const finish = useMutation(finishOidcMutation())
const create = useMutation(createOidcAccountMutation())
const link = useMutation(linkOidcAccountMutation())
const refused = ref('')
const mode = ref<'create' | 'link'>('create')
const username = ref('')
const nickname = ref('')
const password = ref('')
const pending = computed(() => finish.data.value?.pending)

const status = (e: unknown) => (e as { status?: number } | null)?.status
const failure = computed(() => {
  if (refused.value) return refused.value
  switch (status(finish.error.value)) {
    case undefined:
      return ''
    case 403:
      return `Your ${provider.value} login does not have access to Grimoire. Ask the owner for the Grimoire permission, or sign in with your password.`
    case 409:
      return 'That login is already linked to another Account, or this Account already has one.'
    case 410:
      return 'This sign-in expired or was started in another browser. Start again from the sign-in page.'
    default:
      return 'The sign-in could not be finished. Start again from the sign-in page.'
  }
})

function done() {
  void client.invalidateQueries()
  void router.replace(returnTo())
}

onMounted(() => {
  const code = typeof route.query.code === 'string' ? route.query.code : ''
  const state = typeof route.query.state === 'string' ? route.query.state : ''
  // The code is spent once; nothing keeps it in the address bar or the history.
  void router.replace({ name: 'oidc-callback' })
  if (!code || !state) {
    refused.value = 'The sign-in was cancelled or refused. Start again from the sign-in page.'
    return
  }
  finish.mutate({ body: { code, state } }, { onSuccess: (out) => {
      if (out.status === 'signed_in') done()
      if (out.status === 'linked') {
        void client.invalidateQueries()
        void router.replace({ name: 'account' })
      }
      if (out.pending) {
        username.value = out.pending.username.toLowerCase().replace(/[^a-z0-9_.-]/g, '').slice(0, 32)
        nickname.value = (out.pending.name || out.pending.username).slice(0, 40)
      }
    } })
})

function submit() {
  const token = pending.value?.token ?? ''
  if (mode.value === 'create') create.mutate({ body: { token, username: username.value.trim(), nickname: nickname.value.trim() } }, { onSuccess: done })
  else link.mutate({ body: { token, username: username.value.trim(), password: password.value } }, { onSuccess: done })
}
const failed = computed(() => (mode.value === 'create' ? create.error.value : link.error.value))
const failedText = computed(() => {
  switch (status(failed.value)) {
    case 401:
      return 'That Username and password do not match.'
    case 409:
      return mode.value === 'create' ? 'That Username or email already has an Account. Link to it instead.' : 'That Account already has a linked login.'
    case 410:
      return 'This sign-in expired. Start again from the sign-in page.'
    case 422:
      return 'Choose a Username of 3 to 32 lowercase letters, digits, dots, dashes or underscores, and a Nickname.'
    default:
      return 'That did not work. Try again shortly.'
  }
})
</script>

<template>
  <AuthShell>
    <h1>Signing in</h1>
    <template v-if="failure">
      <p role="alert" class="g-alert" data-testid="oidc-failed">{{ failure }}</p>
      <RouterLink :to="{ name: 'sign-in' }">Back to signing in</RouterLink>
    </template>
    <form v-else-if="pending" class="stack" data-testid="oidc-choose" @submit.prevent="submit">
      <p>Welcome, {{ pending.name || pending.username }}. No Grimoire Account has your {{ provider }} login yet.</p>
      <div class="choice" role="radiogroup" aria-label="How to continue">
        <label><input v-model="mode" type="radio" value="create" data-testid="oidc-mode-create" /> Create an Account</label>
        <label><input v-model="mode" type="radio" value="link" data-testid="oidc-mode-link" /> Link to my Account</label>
      </div>
      <template v-if="mode === 'create'">
        <GField v-model="username" label="Username" :maxlength="32" autocomplete="username" required data-testid="oidc-username" />
        <GField v-model="nickname" label="Nickname" :maxlength="40" required data-testid="oidc-nickname" />
        <p class="hint">Your email, {{ pending.email }}, comes from {{ provider }}.</p>
      </template>
      <template v-else>
        <GField v-model="username" label="Username" :maxlength="32" autocomplete="username" required data-testid="oidc-username" />
        <GField v-model="password" label="Password" type="password" :maxlength="200" autocomplete="current-password" required data-testid="oidc-password" />
      </template>
      <p v-if="failed" role="alert" class="g-alert" data-testid="oidc-choose-failed">{{ failedText }}</p>
      <GButton type="submit" variant="primary" :disabled="!username.trim() || (mode === 'create' ? !nickname.trim() : !password) || create.isPending.value || link.isPending.value">
        {{ mode === 'create' ? 'Create the Account' : 'Link and sign in' }}
      </GButton>
    </form>
    <p v-else role="status">Checking your sign-in…</p>
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
.choice {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
  font-size: 14px;
}
</style>
