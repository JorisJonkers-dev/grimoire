<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { createAccountInviteMutation, listAdminAccountsOptions, listAdminAccountsQueryKey } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton } from '@/shared/ui'
import { when } from './labels'
import ReleaseNotesSection from './ReleaseNotesSection.vue'

const client = useQueryClient()
const list = useQuery(listAdminAccountsOptions())
const forbidden = computed(() => (list.error.value as { status?: number } | null)?.status === 403)
const filter = ref('')
const accounts = computed(() => {
  const q = filter.value.trim().toLowerCase()
  const all = list.data.value?.accounts ?? []
  return q ? all.filter((a) => `${a.nickname} ${a.username} ${a.email}`.toLowerCase().includes(q)) : all
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
    <h1>Admin</h1>
    <p v-if="forbidden" role="alert" class="g-alert" data-testid="admin-forbidden">
      Only an Admin with two-step sign-in can open this page.
    </p>
    <p v-else-if="list.isError.value" role="alert" class="g-alert">The Accounts could not be read.</p>
    <template v-else-if="list.data.value">
      <section class="g-card stack" data-testid="admin-accounts">
        <div class="head">
          <h2>Accounts</h2>
          <label class="g-field search">
            <span>Find</span>
            <input v-model="filter" type="search" placeholder="Name, Username or email" data-testid="admin-filter" />
          </label>
        </div>
        <ul class="rows">
          <li v-for="a in accounts" :key="a.id" :data-testid="`admin-account-${a.username}`">
            <RouterLink :to="{ name: 'admin-account', params: { id: a.id } }" class="who">
              <strong>{{ a.nickname }}</strong> <span class="dim">{{ a.username }} · {{ a.email }}</span>
            </RouterLink>
            <span class="chips">
              <span v-if="a.admin" class="chip">Admin</span>
              <span :class="['chip', a.status]">{{ a.status === 'active' ? 'Active' : 'Disabled' }}</span>
              <span class="dim">{{ a.lastSeenAt ? `seen ${when(a.lastSeenAt)}` : 'never seen' }}</span>
            </span>
          </li>
        </ul>
        <p v-if="!accounts.length" data-testid="admin-none">No Account matches.</p>
      </section>
      <section class="g-card stack" data-testid="admin-invites">
        <h2>Invites not yet used</h2>
        <ul v-if="list.data.value.invites.length" class="rows">
          <li v-for="i in list.data.value.invites" :key="i.id">
            <span>{{ i.admin ? 'Admin invite' : 'Invite' }} made {{ when(i.createdAt) }}</span>
            <span :class="['chip', i.status]">{{ i.status === 'invited' ? `open until ${when(i.expiresAt)}` : 'Expired' }}</span>
          </li>
        </ul>
        <p v-else>None.</p>
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
          <GButton type="submit" :disabled="invite.isPending.value">Create an invite</GButton>
          <p v-if="invite.data.value" role="status" data-testid="invite-link">Send this link, which works once: <code>{{ inviteLink(invite.data.value.token) }}</code></p>
        </form>
      </section>
      <ReleaseNotesSection />
    </template>
  </main>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  justify-content: space-between;
  gap: 12px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.search {
  min-width: min(100%, 260px);
}
.rows {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}
.rows li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 0;
  border-bottom: 1px solid var(--color-line);
}
.who {
  color: inherit;
  overflow-wrap: anywhere;
}
.dim {
  color: var(--color-text-3);
  font-size: 14px;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}
.chip {
  padding: 2px 8px;
  border: 1px solid var(--color-line);
  border-radius: 999px;
  font-size: 13px;
}
.chip.disabled,
.chip.expired {
  border-color: var(--color-enemy);
  color: var(--color-enemy-soft);
}
code {
  overflow-wrap: anywhere;
}
</style>
