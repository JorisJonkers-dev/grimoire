<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getSignInMethodsOptions, requestSignInLinkMutation, signInMutation, startOidcSignInMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'
import AuthShell from './AuthShell.vue'
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
// The external login this server offers, by the name it goes by; none when it offers none.
const oidc = computed(() => methods.data.value?.oidc ?? '')
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
  <AuthShell>
    <header class="head">
      <h1>{{ forgot ? 'Sign in with a link' : 'Sign in' }}</h1>
      <p v-if="!challenge && !forgot" class="intro" data-testid="sign-in-intro">
        Use the account an Admin made for you{{ oidc ? `, or your ${oidc} login` : '' }}.
      </p>
    </header>
    <TwoStepForm v-if="challenge" :challenge="challenge" @done="done" @restart="restart" />
    <template v-else-if="!forgot">
      <template v-if="oidc">
        <GButton type="button" class="external" :disabled="external.isPending.value" data-testid="sign-in-oidc" @click="signInExternally">Sign in with {{ oidc }}</GButton>
        <p v-if="external.isError.value" role="alert" class="g-alert" data-testid="sign-in-oidc-failed">{{ oidc }} could not be reached. Try again shortly.</p>
        <p class="or" data-testid="sign-in-or">or with your Username</p>
      </template>
      <form class="stack" data-testid="sign-in-form" @submit.prevent="submit">
        <GField v-model="username" label="Username" :maxlength="32" autocomplete="username" required data-testid="sign-in-username" />
        <GField v-model="password" label="Password" type="password" :maxlength="200" autocomplete="current-password" required data-testid="sign-in-password" />
        <p class="forgot" data-testid="sign-in-forgot">Forgot it? <button type="button" class="link" data-testid="forgot" @click="forgot = true">Email me a sign-in link</button></p>
        <p v-if="signIn.isError.value" role="alert" class="g-alert" data-testid="sign-in-failed">That Username and password do not match.</p>
        <GButton type="submit" variant="primary" :disabled="!username.trim() || !password || signIn.isPending.value">Sign in</GButton>
      </form>
      <p class="note" data-testid="sign-in-note">No account yet? Grimoire is invite-only: ask the person running your table to send you an Account Invite.</p>
    </template>
    <form v-else class="stack" data-testid="link-form" @submit.prevent="sendLink">
      <p class="intro">We email a link that signs you in once, within 30 minutes.</p>
      <GField v-model="email" label="Email" type="email" :maxlength="254" autocomplete="email" required data-testid="link-email" />
      <p v-if="link.isSuccess.value" role="status" data-testid="link-sent">If an Account has that email, a link is on its way.</p>
      <p v-if="link.isError.value" role="alert" class="g-alert">The link could not be sent. Try again shortly.</p>
      <GButton type="submit" variant="primary" :disabled="!email.includes('@') || link.isPending.value">Email me a link</GButton>
      <button type="button" class="link" @click="forgot = false">Back to signing in</button>
    </form>
  </AuthShell>
</template>

<style scoped>
.head {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 34px;
}
.intro {
  margin: 0;
  font-size: 16px;
  color: var(--color-text-2);
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.stack p {
  margin: 0;
}
.external {
  width: 100%;
  min-height: 50px;
  font-size: 18px;
}
/* A rule either side of the words, as a divider between the two ways in. */
.or {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 0;
  font-size: 14px;
  color: var(--color-text-2);
}
.or::before,
.or::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--color-line);
}
.forgot {
  padding-left: 14px;
  font-size: 14px;
  color: var(--color-text-2);
}
.note {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
  color: var(--color-text-2);
}
.link {
  align-self: flex-start;
  padding: 0;
  border: 0;
  background: none;
  font: inherit;
  color: var(--color-gold-high);
  cursor: pointer;
  text-decoration: underline;
}
@media (max-width: 899px) {
  h1 {
    font-size: 26px;
  }
}
</style>
