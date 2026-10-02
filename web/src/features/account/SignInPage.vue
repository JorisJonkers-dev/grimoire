<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getSignInMethodsOptions, requestSignInLinkMutation, signInMutation, startOidcSignInMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'
import { leaveFor } from './leave'
import TwoStepForm from './TwoStepForm.vue'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const username = ref('')
const password = ref('')
const email = ref('')
const forgot = ref(false)
const challenge = ref('')
const signIn = useMutation(signInMutation())
const link = useMutation(requestSignInLinkMutation())
const methods = useQuery(getSignInMethodsOptions())
const external = useMutation(startOidcSignInMutation())
// next only ever leads back inside Grimoire.
const next = () => {
  const n = typeof route.query.next === 'string' ? route.query.next : '/'
  return n.startsWith('/') && !n.startsWith('//') ? n : '/'
}
function done() {
  void client.invalidateQueries()
  void router.push(next())
}
function submit() {
  signIn.mutate({ body: { username: username.value.trim(), password: password.value } }, { onSuccess: (out) => {
        if ('challenge' in out) challenge.value = out.challenge
        else done()
      } })
}
function restart() {
  challenge.value = ''
  password.value = ''
}
function signInExternally() {
  external.mutate({}, { onSuccess: (out) => { leaveFor(out.url, next()) } })
}
function sendLink() {
  link.mutate({ body: { email: email.value.trim() } })
}
</script>

<template>
  <main class="g-page narrow">
    <h1>Sign in</h1>
    <TwoStepForm v-if="challenge" :challenge="challenge" @done="done" @restart="restart" />
    <form v-else-if="!forgot" class="g-card stack" data-testid="sign-in-form" @submit.prevent="submit">
      <GField v-model="username" label="Username" :maxlength="32" autocomplete="username" required data-testid="sign-in-username" />
      <GField v-model="password" label="Password" type="password" :maxlength="200" autocomplete="current-password" required data-testid="sign-in-password" />
      <p v-if="signIn.isError.value" role="alert" class="g-alert" data-testid="sign-in-failed">That Username and password do not match.</p>
      <GButton type="submit" variant="primary" :disabled="!username.trim() || !password || signIn.isPending.value">Sign in</GButton>
      <button type="button" class="link" data-testid="forgot" @click="forgot = true">Forgot your password?</button>
      <template v-if="methods.data.value?.oidc">
        <p class="or" aria-hidden="true">or</p>
        <GButton type="button" :disabled="external.isPending.value" data-testid="sign-in-oidc" @click="signInExternally">Sign in with {{ methods.data.value.oidc }}</GButton>
        <p v-if="external.isError.value" role="alert" class="g-alert">{{ methods.data.value.oidc }} could not be reached. Try again shortly.</p>
      </template>
    </form>
    <form v-else class="g-card stack" data-testid="link-form" @submit.prevent="sendLink">
      <p>We email a link that signs you in once, within 30 minutes.</p>
      <GField v-model="email" label="Email" type="email" :maxlength="254" autocomplete="email" required data-testid="link-email" />
      <p v-if="link.isSuccess.value" role="status" data-testid="link-sent">If an Account has that email, a link is on its way.</p>
      <p v-if="link.isError.value" role="alert" class="g-alert">The link could not be sent. Try again shortly.</p>
      <GButton type="submit" variant="primary" :disabled="!email.includes('@') || link.isPending.value">Email me a link</GButton>
      <button type="button" class="link" @click="forgot = false">Back to signing in</button>
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
.or {
  margin: 0;
  color: var(--color-text-3);
  text-align: center;
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
