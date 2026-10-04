<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import {
  createAccessTokenMutation,
  listAccessTokensOptions,
  listAccessTokensQueryKey,
  revokeAccessTokenMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { AccessTokenScope } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'

const client = useQueryClient()
const tokens = useQuery(listAccessTokensOptions())
const refresh = () => client.invalidateQueries({ queryKey: listAccessTokensQueryKey() })
const mint = useMutation(createAccessTokenMutation())
const revoke = useMutation(revokeAccessTokenMutation())
const name = ref('')
const days = ref(30)
const scopes = ref<AccessTokenScope[]>(['read', 'build'])
const fresh = ref('')
const endpoint = `${window.location.origin}/mcp`
const scopeNames: Record<AccessTokenScope, string> = { read: 'Read everything you can see', build: 'Build Campaigns and their prep', play: 'Act in your Sessions' }
const ready = computed(() => name.value.trim() !== '' && scopes.value.length > 0)
const day = (iso: string) => new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })

function create() {
  mint.mutate({ body: { name: name.value.trim(), scopes: scopes.value, days: days.value } }, { onSuccess: (out) => {
      fresh.value = out.token
      name.value = ''
      void refresh()
    } })
}
function remove(id: string) {
  revoke.mutate({ path: { accessId: id } }, { onSuccess: () => void refresh() })
}
</script>

<template>
  <section class="g-card stack" data-testid="access-tokens">
    <h2>Access Tokens</h2>
    <p>For MCP clients and scripts: a token acts as you, only within its scopes, until it expires or you revoke it. Use it at <code>{{ endpoint }}</code> as <code>Authorization: Bearer &lt;token&gt;</code>.</p>
    <p v-if="fresh" role="status" class="fresh" data-testid="access-token-fresh">
      Copy this token now; it is not shown again: <code>{{ fresh }}</code>
    </p>
    <table v-if="tokens.data.value?.items.length" data-testid="access-token-list">
      <thead>
        <tr>
          <th scope="col">Token</th>
          <th scope="col">Can</th>
          <th scope="col">Expires</th>
          <th scope="col">Last used</th>
          <th scope="col"><span class="sr">Actions</span></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="t in tokens.data.value.items" :key="t.id">
          <td><strong>{{ t.name }}</strong></td>
          <td class="dim">{{ t.scopes.join(', ') }}</td>
          <td>{{ day(t.expiresAt) }}</td>
          <td class="dim">{{ t.lastUsedAt ? `last used ${day(t.lastUsedAt)}` : 'never used' }}</td>
          <td class="act"><GButton type="button" variant="danger" :disabled="revoke.isPending.value" :data-testid="`revoke-${t.id}`" @click="remove(t.id)">Revoke</GButton></td>
        </tr>
      </tbody>
    </table>
    <p v-else-if="tokens.isSuccess.value" data-testid="access-token-none">No Access Tokens yet.</p>
    <h2>A new token</h2>
    <form class="stack" data-testid="access-token-form" @submit.prevent="create">
      <GField v-model="name" label="Name" :maxlength="60" hint="Where it is used, such as Claude on my laptop." data-testid="access-token-name" />
      <fieldset class="scopes">
        <legend>Scopes</legend>
        <label v-for="(text, s) in scopeNames" :key="s"><input v-model="scopes" type="checkbox" :value="s" :data-testid="`scope-${s}`" /> {{ text }}</label>
      </fieldset>
      <label class="g-field">
        <span>Expires after</span>
        <select v-model.number="days" data-testid="access-token-days">
          <option :value="7">A week</option>
          <option :value="30">A month</option>
          <option :value="90">Three months</option>
          <option :value="365">A year</option>
        </select>
      </label>
      <p v-if="mint.isError.value" role="alert" class="g-alert">The token could not be made. Give it a name and at least one scope.</p>
      <GButton type="submit" :disabled="!ready || mint.isPending.value">Make a token</GButton>
    </form>
  </section>
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
p {
  margin: 0;
}
code {
  padding: 2px 6px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-chip);
  font-size: 14px;
  background: var(--color-inset);
  overflow-wrap: anywhere;
}
/* A token just made: said once, plainly, with the token itself to copy. */
.fresh {
  padding: 14px 0 16px;
  border-top: 1px solid var(--color-rule);
  border-bottom: 1px solid var(--color-rule);
  font-weight: 700;
}
.fresh code {
  display: inline-block;
  margin-top: 8px;
  padding: 9px 12px;
  font-weight: 400;
}
.dim {
  color: var(--color-text-2);
}
.act {
  text-align: right;
}
.act :deep(.g-button) {
  min-height: 36px;
  padding: 0 12px;
  font-size: 14px;
}
.sr {
  position: absolute;
  left: -9999px;
}
.scopes {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  border: 0;
}
.scopes legend {
  padding: 0 0 4px;
  font-size: 14px;
  color: var(--color-text-3);
}
.scopes label {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
  border-top: 1px solid var(--color-rule);
}
@media (max-width: 899px) {
  thead {
    display: none;
  }
  tr {
    display: grid;
    gap: 2px;
    padding: 12px 0;
    border-top: 1px solid var(--color-rule);
  }
  td {
    padding: 0;
    border: 0;
  }
  .act {
    text-align: left;
  }
}
@media (pointer: coarse) {
  .act :deep(.g-button) {
    min-height: 44px;
  }
}
</style>
