<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import {
  createAccountInviteMutation,
  getAccountOptions,
  getAccountQueryKey,
  getSignInMethodsOptions,
  setAccountPasswordMutation,
  startOidcLinkMutation,
  unlinkOidcMutation,
  updateAccountMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'
import { leaveFor } from './leave'

const client = useQueryClient()
const account = useQuery(getAccountOptions())
const methods = useQuery(getSignInMethodsOptions())
const refresh = () => client.invalidateQueries({ queryKey: getAccountQueryKey() })
const password = ref('')
const save = useMutation(setAccountPasswordMutation())
const username = ref('')
const nickname = ref('')
const email = ref('')
watch(account.data, (a) => {
  if (!a) return
  username.value = a.username
  nickname.value = a.nickname
  email.value = a.email
}, { immediate: true })
const profile = useMutation(updateAccountMutation())
function saveProfile() {
  profile.mutate({ body: { username: username.value.trim(), nickname: nickname.value.trim(), email: email.value.trim() } }, { onSuccess: () => void refresh() })
}
function savePassword() {
  save.mutate({ body: { password: password.value } }, { onSuccess: () => {
      password.value = ''
      void refresh()
    } })
}
const linking = useMutation(startOidcLinkMutation())
const unlinking = useMutation(unlinkOidcMutation())
function link() {
  linking.mutate({}, { onSuccess: (out) => { leaveFor(out.url, '/account') } })
}
function unlink() {
  unlinking.mutate({}, { onSuccess: () => void refresh() })
}
const hours = ref(72)
const asAdmin = ref(false)
const invite = useMutation(createAccountInviteMutation())
const inviteLink = (token: string) => `${window.location.origin}/account-invite#${token}`
</script>

<template>
  <main class="g-page">
    <h1>Your Account</h1>
    <p v-if="account.isError.value" role="alert" class="g-alert">Your Account could not be read.</p>
    <template v-else-if="account.data.value">
      <form class="g-card stack" data-testid="profile-form" @submit.prevent="saveProfile">
        <h2>Profile</h2>
        <p v-if="account.data.value.admin">You are an Admin.</p>
        <GField v-model="username" label="Username" :maxlength="32" autocomplete="username" required data-testid="profile-username" />
        <GField v-model="nickname" label="Nickname" :maxlength="40" required data-testid="profile-nickname" />
        <GField v-model="email" label="Email" type="email" :maxlength="254" autocomplete="email" required data-testid="profile-email" />
        <p v-if="profile.isSuccess.value" role="status" data-testid="profile-saved">Saved.</p>
        <p v-if="profile.isError.value" role="alert" class="g-alert" data-testid="profile-failed">
          {{ (profile.error.value as { status?: number } | null)?.status === 409 ? 'That Username or email already has an Account.' : 'Choose a Username of 3 to 32 lowercase letters, digits, dots, dashes or underscores, a Nickname and an email.' }}
        </p>
        <GButton type="submit" :disabled="!username.trim() || !nickname.trim() || !email.includes('@') || profile.isPending.value">Save the profile</GButton>
      </form>
      <section v-if="account.data.value.oidc || methods.data.value?.oidc" class="g-card stack" data-testid="oidc-section">
        <h2>{{ methods.data.value?.oidc ?? 'External login' }}</h2>
        <template v-if="account.data.value.oidc">
          <p>Linked. These come from {{ methods.data.value?.oidc ?? 'your login' }} and change only there.</p>
          <dl class="readonly" data-testid="oidc-fields">
            <dt>Name</dt><dd>{{ account.data.value.oidc.name || '—' }}</dd>
            <dt>Username</dt><dd>{{ account.data.value.oidc.username || '—' }}</dd>
            <dt>Email</dt><dd>{{ account.data.value.oidc.email || '—' }}</dd>
          </dl>
          <p v-if="!account.data.value.hasPassword" class="hint" data-testid="unlink-needs-password">Set a password below before unlinking, so you can still sign in.</p>
          <p v-if="unlinking.isError.value" role="alert" class="g-alert">The login could not be unlinked.</p>
          <GButton type="button" :disabled="!account.data.value.hasPassword || unlinking.isPending.value" data-testid="oidc-unlink" @click="unlink">Unlink</GButton>
        </template>
        <template v-else>
          <p>Link your {{ methods.data.value?.oidc }} login to sign in with it as this Account.</p>
          <p v-if="linking.isError.value" role="alert" class="g-alert">{{ methods.data.value?.oidc }} could not be reached. Try again shortly.</p>
          <GButton type="button" :disabled="linking.isPending.value" data-testid="oidc-link" @click="link">Link {{ methods.data.value?.oidc }}</GButton>
        </template>
      </section>
      <form class="g-card stack" data-testid="password-form" @submit.prevent="savePassword">
        <h2>Password</h2>
        <GField v-model="password" :label="account.data.value.hasPassword ? 'New password' : 'Password'" type="password" :maxlength="200" autocomplete="new-password" data-testid="account-password" />
        <p v-if="save.isSuccess.value" role="status" data-testid="password-saved">Saved.</p>
        <GButton type="submit" :disabled="password.length < 10 || save.isPending.value">{{ account.data.value.hasPassword ? 'Change the password' : 'Set a password' }}</GButton>
      </form>
      <form v-if="account.data.value.admin" class="g-card stack" data-testid="invite-form" @submit.prevent="invite.mutate({ body: { hours, admin: asAdmin } })">
        <h2>Invite someone</h2>
        <label class="g-field">
          <span>Open for</span>
          <select v-model.number="hours" data-testid="invite-hours">
            <option :value="24">A day</option>
            <option :value="72">Three days</option>
            <option :value="168">A week</option>
          </select>
        </label>
        <label class="check"><input v-model="asAdmin" type="checkbox" data-testid="invite-admin" /><span>As an Admin</span></label>
        <GButton type="submit" :disabled="invite.isPending.value">Create the invite</GButton>
        <p v-if="invite.data.value" role="status" data-testid="invite-link">Send this link, which works once: <code>{{ inviteLink(invite.data.value.token) }}</code></p>
      </form>
    </template>
  </main>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.readonly {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 16px;
  margin: 0;
}
.readonly dt {
  color: var(--color-text-3);
}
.readonly dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
  font-size: 14px;
}
</style>
