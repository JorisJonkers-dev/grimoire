<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  getAdminAccountOptions,
  getAdminAccountQueryKey,
  resetAccountTwoStepMutation,
  sendAdminSignInLinkMutation,
  setAccountDisabledMutation,
  setAdminRoleMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GAvatar, GButton } from '@/shared/ui'
import { eventLabels, when } from './labels'

const route = useRoute()
const client = useQueryClient()
const path = computed(() => ({ accountId: String(route.params.id) }))
const detail = useQuery(computed(() => getAdminAccountOptions({ path: path.value })))
const refresh = () => client.invalidateQueries({ queryKey: getAdminAccountQueryKey({ path: path.value }) })
const link = useMutation(sendAdminSignInLinkMutation())
const role = useMutation(setAdminRoleMutation())
const disable = useMutation(setAccountDisabledMutation())
const reset = useMutation(resetAccountTwoStepMutation())
const done = ref('')
const failed = computed(() => [link, role, disable, reset].find((m) => m.isError.value)?.error.value as { status?: number } | null | undefined)
const failure = computed(() => (failed.value?.status === 409 ? 'You cannot do that to your own Account, or to a disabled one.' : 'That did not work. Try again shortly.'))

function after(message: string) {
  return { onSuccess: () => {
    done.value = message
    void refresh()
  } }
}
</script>

<template>
  <main class="g-page">
    <nav aria-label="Breadcrumb" class="g-crumbs">
      <RouterLink :to="{ name: 'admin' }">Admin</RouterLink> <span aria-hidden="true">›</span>
      <RouterLink :to="{ name: 'admin' }">Accounts</RouterLink> <span aria-hidden="true">›</span>
      <span aria-current="page">{{ detail.data.value?.account.nickname ?? 'Account' }}</span>
    </nav>
    <p v-if="detail.isError.value" role="alert" class="g-alert" data-testid="admin-account-error">This Account could not be read.</p>
    <template v-else-if="detail.data.value">
      <header class="head" data-testid="admin-account-summary">
        <GAvatar :name="detail.data.value.account.nickname" :size="64" aria-hidden="true" />
        <div class="who">
          <h1>{{ detail.data.value.account.nickname }} <span class="handle">@{{ detail.data.value.account.username }}</span></h1>
          <p class="g-meta">
            <span class="state"><span :class="['dot', detail.data.value.status]" aria-hidden="true" />{{ detail.data.value.status === 'active' ? 'Active' : 'Disabled' }}</span>
            <span>{{ detail.data.value.account.admin ? 'Admin' : 'Member' }}</span>
            <span>Joined {{ when(detail.data.value.createdAt) }}</span>
          </p>
        </div>
      </header>
      <div class="cols">
        <div class="main">
          <section>
            <h2>Account</h2>
            <dl class="rows" data-testid="admin-account-fields">
              <dt>Username</dt><dd>{{ detail.data.value.account.username }}</dd>
              <dt>Nickname</dt><dd>{{ detail.data.value.account.nickname }}</dd>
              <dt>Email</dt><dd>{{ detail.data.value.account.email }}</dd>
            </dl>
          </section>
          <section>
            <h2>Sign-in</h2>
            <dl class="rows" data-testid="admin-account-methods">
              <dt>Password</dt>
              <dd>{{ detail.data.value.account.hasPassword ? 'A password' : 'No password' }}{{ detail.data.value.account.twoStep ? ', with two-step' : '' }}</dd>
              <dt>External login</dt>
              <dd v-if="detail.data.value.account.oidc">{{ detail.data.value.account.oidc.name || detail.data.value.account.oidc.username }} ({{ detail.data.value.account.oidc.email }})</dd>
              <dd v-else class="dim">Not linked</dd>
              <dt>Signed in on</dt><dd>{{ detail.data.value.sessions }} signed-in devices</dd>
              <dt>Access Tokens</dt><dd>{{ detail.data.value.tokens }} Access Tokens</dd>
            </dl>
          </section>
          <section data-testid="admin-account-campaigns">
            <h2>Campaigns</h2>
            <ul v-if="detail.data.value.campaigns.length" class="g-list">
              <li v-for="c in detail.data.value.campaigns" :key="c.id">{{ c.name }} · <span class="dim">{{ c.role === 'dm' ? 'DM' : 'Player' }}</span></li>
            </ul>
            <p v-else class="dim">None.</p>
          </section>
          <section data-testid="admin-account-history">
            <h2>History</h2>
            <ol class="g-list">
              <li v-for="(e, i) in detail.data.value.history" :key="i" class="event">
                <span>{{ eventLabels[e.action] }}<template v-if="e.detail"> ({{ e.detail }})</template></span>
                <span class="dim">{{ e.actor }} · {{ when(e.at) }}</span>
              </li>
            </ol>
          </section>
        </div>
        <aside aria-label="Controls" class="side" data-testid="admin-account-controls">
          <section>
            <h2>Role</h2>
            <p class="note">Admins manage Accounts, Release Notes and the Shared Library.</p>
            <GButton
              type="button"
              :disabled="role.isPending.value"
              data-testid="admin-toggle-admin"
              @click="role.mutate({ path, body: { value: !detail.data.value.account.admin } }, after('The Admin role changed.'))"
            >
              {{ detail.data.value.account.admin ? 'Remove the Admin role' : 'Make an Admin' }}
            </GButton>
          </section>
          <section>
            <h2>Sign-in</h2>
            <GButton type="button" :disabled="link.isPending.value" data-testid="admin-send-link" @click="link.mutate({ path }, after('A sign-in link is on its way.'))">Email a sign-in link</GButton>
            <p class="note small">For a forgotten password.</p>
            <GButton v-if="detail.data.value.account.twoStep" type="button" :disabled="reset.isPending.value" data-testid="admin-reset-two-step" @click="reset.mutate({ path }, after('Two-step is off; they can set it up again.'))">
              Reset two-step
            </GButton>
          </section>
          <section>
            <h2 class="danger">Access</h2>
            <p class="note">Disabling ends every session and Access Token. Characters and Campaigns are kept.</p>
            <GButton
              type="button"
              :variant="detail.data.value.status === 'active' ? 'danger' : 'secondary'"
              :disabled="disable.isPending.value"
              data-testid="admin-toggle-disabled"
              @click="disable.mutate({ path, body: { value: detail.data.value.status === 'active' } }, after(detail.data.value.status === 'active' ? 'Disabled; every session and Access Token has ended.' : 'Enabled again.'))"
            >
              {{ detail.data.value.status === 'active' ? 'Disable' : 'Enable' }}
            </GButton>
          </section>
          <p v-if="failed" role="alert" class="g-alert" data-testid="admin-control-failed">{{ failure }}</p>
          <p v-else-if="done" role="status" data-testid="admin-control-done">{{ done }}</p>
        </aside>
      </div>
    </template>
  </main>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 20px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--color-line);
}
.who {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
h1 {
  margin: 0;
  font-size: 32px;
  overflow-wrap: anywhere;
}
.handle {
  font-family: var(--font-ui);
  font-size: 18px;
  font-weight: 400;
  letter-spacing: 0;
  text-transform: none;
  color: var(--color-text-3);
}
.state {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-success);
}
.dot.disabled {
  background: var(--color-danger-edge);
}
.cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 36px 72px;
  align-items: start;
}
.main {
  display: flex;
  flex-direction: column;
  gap: 36px;
  min-width: 0;
}
.side {
  display: flex;
  flex-direction: column;
}
.side section {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  padding: 22px 0;
  border-top: 1px solid var(--color-rule);
}
.side section:first-child {
  padding-top: 0;
  border-top: 0;
}
h2 {
  margin: 0 0 10px;
}
.side h2 {
  margin: 0;
}
h2.danger {
  color: var(--color-danger-text);
}
.rows {
  display: grid;
  grid-template-columns: 200px minmax(0, 1fr);
  margin: 0;
  font-size: 15px;
}
.rows dt,
.rows dd {
  margin: 0;
  padding: 11px 0;
  border-top: 1px solid var(--color-rule);
  overflow-wrap: anywhere;
}
.rows dt {
  color: var(--color-text-3);
}
.event {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
}
.dim {
  color: var(--color-text-3);
}
.note {
  margin: 0;
  font-size: 15px;
  color: var(--color-text-2);
}
.note.small {
  font-size: 13px;
  color: var(--color-text-3);
}
p {
  margin: 0;
}
@media (max-width: 899px) {
  .cols {
    grid-template-columns: minmax(0, 1fr);
  }
  .rows {
    grid-template-columns: minmax(0, 1fr);
  }
  .rows dd {
    padding-top: 0;
    border-top: 0;
  }
  .rows dt {
    padding-bottom: 2px;
  }
}
</style>
