<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
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
import { GAvatar, GButton, GTabs } from '@/shared/ui'
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
// The page is four tabs; every tab stays on the page so a half-filled form survives a look elsewhere.
const tab = ref('profile')
const tabs = computed(() => [
  { value: 'profile', label: 'Profile & sign-in' },
  { value: 'notifications', label: 'Notifications' },
  { value: 'tokens', label: 'Access Tokens' },
  { value: 'activity', label: 'Activity' },
])
</script>

<template>
  <main class="g-page">
    <h1 v-if="!account.data.value">Account</h1>
    <p v-if="account.isError.value" role="alert" class="g-alert">Your Account could not be read.</p>
    <template v-else-if="account.data.value">
      <header class="head" data-testid="account-head">
        <GAvatar :name="account.data.value.nickname" :size="64" />
        <div class="who">
          <h1>Account</h1>
          <p data-testid="account-who">
            {{ account.data.value.nickname }} <span class="dim">@{{ account.data.value.username }}</span><template v-if="account.data.value.admin"> · Admin</template>
          </p>
        </div>
      </header>
      <GTabs v-model="tab" label="Account" :tabs="tabs">
        <div v-show="tab === 'profile'" class="tab" data-testid="account-tab-profile">
          <form class="part" data-testid="profile-form" @submit.prevent="saveProfile">
            <h2>Profile</h2>
            <div class="g-rows">
              <div class="g-rows__row">
                <label for="profile-username">Username</label>
                <input id="profile-username" v-model="username" class="g-input" maxlength="32" autocomplete="username" required data-testid="profile-username" />
              </div>
              <div class="g-rows__row">
                <label for="profile-nickname">Nickname</label>
                <input id="profile-nickname" v-model="nickname" class="g-input" maxlength="40" required data-testid="profile-nickname" />
              </div>
              <div class="g-rows__row">
                <label for="profile-email">Email</label>
                <input id="profile-email" v-model="email" class="g-input" type="email" maxlength="254" autocomplete="email" required data-testid="profile-email" />
              </div>
              <div v-if="account.data.value.admin" class="g-rows__row">
                <span>Role</span>
                <span>You are an Admin.</span>
                <RouterLink v-if="account.data.value.adminPowers" :to="{ name: 'admin' }" class="g-action" data-testid="account-admin-link">Manage Accounts and invites</RouterLink>
              </div>
            </div>
            <p v-if="profile.isSuccess.value" role="status" data-testid="profile-saved">Saved.</p>
            <p v-if="profile.isError.value" role="alert" class="g-alert" data-testid="profile-failed">
              {{ (profile.error.value as { status?: number } | null)?.status === 409 ? 'That Username or email already has an Account.' : 'Choose a Username of 3 to 32 lowercase letters, digits, dots, dashes or underscores, a Nickname and an email address.' }}
            </p>
            <GButton type="submit" variant="primary" :disabled="!username.trim() || !nickname.trim() || !email.includes('@') || profile.isPending.value">Save the profile</GButton>
          </form>
          <section class="part" data-testid="sign-in-methods">
            <h2>Sign-in methods</h2>
            <div class="g-rows">
              <div v-if="account.data.value.oidc || methods.data.value?.oidc" class="g-rows__row" data-testid="oidc-section">
                <span>{{ methods.data.value?.oidc ?? 'External login' }}</span>
                <template v-if="account.data.value.oidc">
                  <div class="value">
                    <span class="state"><span class="dot" aria-hidden="true" />Linked</span>
                    <p class="hint">These come from {{ methods.data.value?.oidc ?? 'your login' }} and change only there.</p>
                    <dl class="readonly" data-testid="oidc-fields">
                      <dt>Name</dt><dd>{{ account.data.value.oidc.name || '—' }}</dd>
                      <dt>Username</dt><dd>{{ account.data.value.oidc.username || '—' }}</dd>
                      <dt>Email</dt><dd>{{ account.data.value.oidc.email || '—' }}</dd>
                    </dl>
                    <p v-if="!account.data.value.hasPassword" class="hint" data-testid="unlink-needs-password">Set a password below before unlinking, so you can still sign in.</p>
                    <p v-if="unlinking.isError.value" role="alert" class="g-alert">The login could not be unlinked.</p>
                  </div>
                  <GButton type="button" :disabled="!account.data.value.hasPassword || unlinking.isPending.value" data-testid="oidc-unlink" @click="unlink">Unlink</GButton>
                </template>
                <template v-else>
                  <div class="value">
                    <span>Not linked</span>
                    <p class="hint">Link your {{ methods.data.value?.oidc }} login to sign in with it as this Account.</p>
                    <p v-if="linking.isError.value" role="alert" class="g-alert">{{ methods.data.value?.oidc }} could not be reached. Try again shortly.</p>
                  </div>
                  <GButton type="button" :disabled="linking.isPending.value" data-testid="oidc-link" @click="link">Link {{ methods.data.value?.oidc }}</GButton>
                </template>
              </div>
              <form class="g-rows__row" data-testid="password-form" @submit.prevent="savePassword">
                <label for="account-password">Password</label>
                <div class="value">
                  <span>{{ account.data.value.hasPassword ? 'Set' : 'Not set' }}</span>
                  <input
                    id="account-password"
                    v-model="password"
                    class="g-input"
                    type="password"
                    maxlength="200"
                    autocomplete="new-password"
                    :placeholder="account.data.value.hasPassword ? 'A new password, 10 characters or more' : 'A password, 10 characters or more'"
                    data-testid="account-password"
                  />
                  <p v-if="save.isSuccess.value" role="status" data-testid="password-saved">Saved.</p>
                </div>
                <GButton type="submit" :disabled="password.length < 10 || save.isPending.value">{{ account.data.value.hasPassword ? 'Change the password' : 'Set a password' }}</GButton>
              </form>
              <div class="g-rows__row g-rows__row--wide">
                <span>Two-step sign-in</span>
                <TwoStepSection v-if="account.data.value.hasPassword" :account="account.data.value" />
                <span v-else class="hint">Two-step sign-in guards a password; set one first.</span>
              </div>
            </div>
            <p class="hint foot">A method can be removed only while another one remains.</p>
          </section>
        </div>
        <div v-show="tab === 'notifications'" class="tab tab--wide" data-testid="account-tab-notifications">
          <NotificationPreferencesSection />
        </div>
        <div v-show="tab === 'tokens'" class="tab tab--wide" data-testid="account-tab-tokens">
          <AccessTokensSection />
        </div>
        <div v-show="tab === 'activity'" class="tab" data-testid="account-tab-activity">
          <section class="part">
            <h2>Recent activity</h2>
            <ol v-if="history.data.value?.items.length" class="g-list" data-testid="account-history">
              <li v-for="(e, i) in history.data.value.items" :key="i" class="event">
                <span>{{ eventLabels[e.action] }}</span>
                <span class="dim">{{ e.actor }} · {{ when(e.at) }}</span>
              </li>
            </ol>
            <p v-else class="hint">Nothing has happened to this Account yet.</p>
          </section>
        </div>
      </GTabs>
    </template>
  </main>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 28px;
}
.head :deep(.g-avatar) {
  border-color: var(--color-brass-line);
  color: var(--color-bar-brand);
  background: var(--color-bar-avatar);
}
.who {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.who h1,
.who p {
  margin: 0;
}
.who p {
  font-size: 15px;
  color: var(--color-text-2);
  overflow-wrap: anywhere;
}
.dim {
  color: var(--color-text-3);
}
.tab {
  display: flex;
  flex-direction: column;
  gap: 36px;
  max-width: 880px;
  padding-top: 28px;
}
.tab--wide {
  max-width: none;
}
.part {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}
.part > .g-rows,
.part > .g-list {
  align-self: stretch;
}
.part h2 {
  margin: 0;
}
.value {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  min-width: 0;
}
.value p {
  margin: 0;
}
.state {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.dot {
  width: 7px;
  height: 7px;
  background: var(--color-success);
  transform: rotate(45deg);
}
.readonly {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 2px 16px;
  margin: 0;
}
.readonly dt {
  color: var(--color-text-3);
}
.readonly dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.event {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
}
.hint {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-3);
}
.foot {
  align-self: stretch;
  padding-top: 12px;
  border-top: 1px solid var(--color-rule);
}
</style>
