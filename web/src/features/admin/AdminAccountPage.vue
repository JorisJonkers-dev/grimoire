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
import { GButton } from '@/shared/ui'
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
    <RouterLink :to="{ name: 'admin' }">All Accounts</RouterLink>
    <p v-if="detail.isError.value" role="alert" class="g-alert" data-testid="admin-account-error">This Account could not be read.</p>
    <template v-else-if="detail.data.value">
      <h1>{{ detail.data.value.account.nickname }}</h1>
      <section class="g-card stack" data-testid="admin-account-summary">
        <p>{{ detail.data.value.account.username }} · {{ detail.data.value.account.email }}</p>
        <p>
          <span :class="['chip', detail.data.value.status]">{{ detail.data.value.status === 'active' ? 'Active' : 'Disabled' }}</span>
          <span v-if="detail.data.value.account.admin" class="chip">Admin</span>
          · created {{ when(detail.data.value.createdAt) }}
        </p>
        <h2>Signs in with</h2>
        <ul data-testid="admin-account-methods">
          <li>{{ detail.data.value.account.hasPassword ? 'A password' : 'No password' }}{{ detail.data.value.account.twoStep ? ', with two-step' : '' }}</li>
          <li v-if="detail.data.value.account.oidc">External login: {{ detail.data.value.account.oidc.name || detail.data.value.account.oidc.username }} ({{ detail.data.value.account.oidc.email }})</li>
          <li>{{ detail.data.value.sessions }} signed-in devices · {{ detail.data.value.tokens }} Access Tokens</li>
        </ul>
      </section>
      <section class="g-card stack" data-testid="admin-account-controls">
        <h2>Controls</h2>
        <div class="row">
          <GButton type="button" :disabled="link.isPending.value" data-testid="admin-send-link" @click="link.mutate({ path }, after('A sign-in link is on its way.'))">Email a sign-in link</GButton>
          <GButton
            type="button"
            :disabled="role.isPending.value"
            data-testid="admin-toggle-admin"
            @click="role.mutate({ path, body: { value: !detail.data.value.account.admin } }, after('The Admin role changed.'))"
          >
            {{ detail.data.value.account.admin ? 'Remove the Admin role' : 'Make an Admin' }}
          </GButton>
          <GButton v-if="detail.data.value.account.twoStep" type="button" :disabled="reset.isPending.value" data-testid="admin-reset-two-step" @click="reset.mutate({ path }, after('Two-step is off; they can set it up again.'))">
            Reset two-step
          </GButton>
          <GButton
            type="button"
            :variant="detail.data.value.status === 'active' ? 'danger' : 'secondary'"
            :disabled="disable.isPending.value"
            data-testid="admin-toggle-disabled"
            @click="disable.mutate({ path, body: { value: detail.data.value.status === 'active' } }, after(detail.data.value.status === 'active' ? 'Disabled; every session and Access Token has ended.' : 'Enabled again.'))"
          >
            {{ detail.data.value.status === 'active' ? 'Disable' : 'Enable' }}
          </GButton>
        </div>
        <p v-if="failed" role="alert" class="g-alert" data-testid="admin-control-failed">{{ failure }}</p>
        <p v-else-if="done" role="status" data-testid="admin-control-done">{{ done }}</p>
      </section>
      <section class="g-card stack" data-testid="admin-account-campaigns">
        <h2>Campaigns</h2>
        <ul v-if="detail.data.value.campaigns.length">
          <li v-for="c in detail.data.value.campaigns" :key="c.id">{{ c.name }} · {{ c.role === 'dm' ? 'DM' : 'Player' }}</li>
        </ul>
        <p v-else>None.</p>
      </section>
      <section class="g-card stack" data-testid="admin-account-history">
        <h2>History</h2>
        <ol class="history">
          <li v-for="(e, i) in detail.data.value.history" :key="i">
            <span>{{ eventLabels[e.action] }}<template v-if="e.detail"> ({{ e.detail }})</template></span>
            <span class="dim">{{ e.actor }} · {{ when(e.at) }}</span>
          </li>
        </ol>
      </section>
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
p,
ul {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.chip {
  padding: 2px 8px;
  border: 1px solid var(--color-line);
  border-radius: 999px;
  font-size: 13px;
}
.chip.disabled {
  border-color: var(--color-enemy);
  color: var(--color-enemy-soft);
}
.history {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
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
</style>
