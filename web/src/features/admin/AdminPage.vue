<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createAccountInviteMutation, listAdminAccountsOptions, listAdminAccountsQueryKey } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GAvatar, GButton, GPageHead } from '@/shared/ui'
import AdminTabs from './AdminTabs.vue'
import { when } from './labels'
import ReleaseNotesSection from './ReleaseNotesSection.vue'

const client = useQueryClient()
const route = useRoute()
const router = useRouter()
const list = useQuery(listAdminAccountsOptions())
const forbidden = computed(() => (list.error.value as { status?: number } | null)?.status === 403)
const titles = { accounts: 'Accounts', invites: 'Invites', notes: 'Release Notes' } as const
const tab = computed(() => (route.query.tab === 'invites' || route.query.tab === 'notes' ? route.query.tab : 'accounts'))
const filter = ref('')
const accounts = computed(() => {
  const q = filter.value.trim().toLowerCase()
  const all = list.data.value?.accounts ?? []
  return q ? all.filter((a) => `${a.nickname} ${a.username} ${a.email}`.toLowerCase().includes(q)) : all
})
const open = computed(() => list.data.value?.invites.filter((i) => i.status === 'invited').length ?? 0)
const plural = (n: number, one: string, many = `${one}s`) => `${String(n)} ${n === 1 ? one : many}`
const tally = computed(() => {
  const all = list.data.value?.accounts ?? []
  const off = all.filter((a) => a.status === 'disabled').length
  return [plural(all.length, 'account'), `${plural(open.value, 'invite')} not yet used`, ...(off ? [`${String(off)} disabled`] : [])].join(' · ')
})
const hours = ref(72)
const asAdmin = ref(false)
const invite = useMutation(createAccountInviteMutation())
const inviteLink = (token: string) => `${window.location.origin}/account-invite#${token}`
function send() {
  invite.mutate({ body: { hours: hours.value, admin: asAdmin.value } }, { onSuccess: () => void client.invalidateQueries({ queryKey: listAdminAccountsQueryKey() }) })
}
</script>

<template>
  <main class="g-page">
    <GPageHead eyebrow="Admin" :title="titles[tab]">
      <GButton v-if="list.data.value && tab !== 'invites'" variant="primary" data-testid="new-invite" @click="router.push({ name: 'admin', query: { tab: 'invites' } })">New invite</GButton>
    </GPageHead>
    <AdminTabs :current="tab" :accounts="list.data.value?.accounts.length" :invites="list.data.value ? open : undefined" />
    <p v-if="forbidden" role="alert" class="g-alert" data-testid="admin-forbidden">
      Only an Admin with two-step sign-in can open this page.
    </p>
    <p v-else-if="list.isError.value" role="alert" class="g-alert">The Accounts could not be read.</p>
    <template v-else-if="list.data.value">
      <section v-show="tab === 'accounts'" class="part" data-testid="admin-accounts">
        <div class="bar">
          <p class="tally" data-testid="admin-tally">{{ tally }}</p>
          <label class="find">
            <svg width="15" height="15" viewBox="0 0 16 16" aria-hidden="true"><circle cx="7" cy="7" r="5" fill="none" stroke="currentColor" stroke-width="1.6" /><path d="M11 11 L14.5 14.5" stroke="currentColor" stroke-width="1.6" /></svg>
            <input v-model="filter" type="search" placeholder="Name, Username or email" aria-label="Find an Account" data-testid="admin-filter" />
          </label>
        </div>
        <table v-if="accounts.length">
          <thead>
            <tr>
              <th scope="col">Account</th>
              <th scope="col">Email</th>
              <th scope="col">Role</th>
              <th scope="col">Status</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="a in accounts" :key="a.id" :data-testid="`admin-account-${a.username}`">
              <td>
                <span class="who">
                  <GAvatar :name="a.nickname" :size="34" />
                  <span class="names">
                    <RouterLink :to="{ name: 'admin-account', params: { id: a.id } }">{{ a.nickname }}</RouterLink>
                    <span class="dim">@{{ a.username }}</span>
                  </span>
                </span>
              </td>
              <td class="email">{{ a.email }}</td>
              <td><span :class="{ admin: a.admin }">{{ a.admin ? 'Admin' : 'Member' }}</span></td>
              <td>
                <span class="state"><span :class="['dot', a.status]" aria-hidden="true" />{{ a.status === 'active' ? 'Active' : 'Disabled' }}</span>
                <span class="dim seen">{{ a.lastSeenAt ? `seen ${when(a.lastSeenAt)}` : 'never seen' }}</span>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-else data-testid="admin-none">No Account matches.</p>
      </section>
      <section v-show="tab === 'invites'" class="part" data-testid="admin-invites">
        <h2>Not yet used</h2>
        <ul v-if="list.data.value.invites.length" class="g-list">
          <li v-for="i in list.data.value.invites" :key="i.id" class="invite">
            <span>{{ i.admin ? 'Admin invite' : 'Invite' }} made {{ when(i.createdAt) }}</span>
            <span class="state"><span :class="['dot', i.status]" aria-hidden="true" />{{ i.status === 'invited' ? `open until ${when(i.expiresAt)}` : 'Expired' }}</span>
          </li>
        </ul>
        <p v-else class="dim">None.</p>
        <h2>A new invite</h2>
        <form class="stack" data-testid="invite-form" @submit.prevent="send">
          <label class="g-field">
            <span>Open for</span>
            <select v-model.number="hours" data-testid="invite-hours">
              <option :value="24">A day</option>
              <option :value="72">Three days</option>
              <option :value="168">A week</option>
            </select>
          </label>
          <label class="check"><input v-model="asAdmin" type="checkbox" data-testid="invite-admin" /><span>As an Admin</span></label>
          <GButton type="submit" variant="primary" :disabled="invite.isPending.value">Create an invite</GButton>
          <p v-if="invite.data.value" role="status" data-testid="invite-link">Send this link, which works once: <code>{{ inviteLink(invite.data.value.token) }}</code></p>
        </form>
      </section>
      <ReleaseNotesSection v-show="tab === 'notes'" />
    </template>
  </main>
</template>

<style scoped>
.part {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.part h2 {
  margin: 0;
}
.bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.tally {
  margin: 0;
  font-size: 15px;
  color: var(--color-text-2);
}
.find {
  display: flex;
  align-items: center;
  gap: 10px;
  box-sizing: border-box;
  width: min(100%, 300px);
  min-height: var(--size-control);
  padding: 0 12px;
  border-radius: var(--radius-control);
  color: var(--color-text-3);
  background: var(--color-inset);
}
.find:focus-within {
  box-shadow: inset 0 -2px 0 var(--color-brass-edge);
}
.find input {
  flex: 1;
  min-width: 0;
  border: 0;
  font: inherit;
  font-size: 15px;
  color: var(--color-text);
  background: transparent;
}
.find input:focus {
  outline: none;
}
.who {
  display: flex;
  align-items: center;
  gap: 12px;
}
.names {
  display: flex;
  flex-direction: column;
}
.dim {
  font-size: 13px;
  color: var(--color-text-3);
}
.email {
  overflow-wrap: anywhere;
}
.admin {
  font-family: var(--font-label);
  color: var(--color-gold-high);
}
.state {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.seen {
  display: block;
}
.dot {
  width: 7px;
  height: 7px;
  background: var(--color-success);
  transform: rotate(45deg);
}
.dot.disabled,
.dot.expired {
  background: var(--color-danger-edge);
}
.invite {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 420px;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
code {
  overflow-wrap: anywhere;
}
/* On a phone each Account is a block: who, then the rest beneath. */
@media (max-width: 899px) {
  thead {
    display: none;
  }
  tr {
    display: grid;
    gap: 4px;
    padding: 12px 0;
    border-top: 1px solid var(--color-rule);
  }
  td {
    padding: 0 0 0 46px;
    border: 0;
  }
  td:first-child {
    padding-left: 0;
  }
}
</style>
