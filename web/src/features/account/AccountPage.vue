<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import {
  getAccountOptions,
  getAccountHistoryOptions,
  getAccountQueryKey,
  getSignInMethodsOptions,
  setAccountPasswordMutation,
  startOidcLinkMutation,
  unlinkOidcMutation,
  updateAccountMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'
import { leaveFor } from './leave'
import { eventLabels, when } from '@/features/admin/labels'
import NotificationPreferencesSection from '@/features/notifications/NotificationPreferencesSection.vue'
import AccessTokensSection from './AccessTokensSection.vue'
import TwoStepSection from './TwoStepSection.vue'

const client = useQueryClient()
const account = useQuery(getAccountOptions())
const methods = useQuery(getSignInMethodsOptions())
const history = useQuery(getAccountHistoryOptions())
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
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">Account</span>
      <h1>Your Account</h1>
    </header>
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
      <TwoStepSection v-if="account.data.value.hasPassword" :account="account.data.value" />
      <NotificationPreferencesSection />
      <AccessTokensSection />
      <p v-if="account.data.value.adminPowers" class="g-card"><RouterLink :to="{ name: 'admin' }" data-testid="account-admin-link">Manage Accounts and invites</RouterLink></p>
      <details v-if="history.data.value?.items.length" class="g-card" data-testid="account-history">
        <summary>Recent activity</summary>
        <ol class="history">
          <li v-for="(e, i) in history.data.value.items" :key="i">
            <span>{{ eventLabels[e.action] }}</span>
            <span class="dim">{{ e.actor }} · {{ when(e.at) }}</span>
          </li>
        </ol>
      </details>
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
.history {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}
.history li {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
}
.dim {
  color: var(--color-text-3);
  font-size: 14px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
  font-size: 14px;
}
</style>
