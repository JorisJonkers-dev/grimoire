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
    <ul v-if="tokens.data.value?.items.length" class="list" data-testid="access-token-list">
      <li v-for="t in tokens.data.value.items" :key="t.id">
        <div>
          <strong>{{ t.name }}</strong> · {{ t.scopes.join(', ') }}
          <div class="meta">
            Expires {{ day(t.expiresAt) }} · {{ t.lastUsedAt ? `last used ${day(t.lastUsedAt)}` : 'never used' }}
          </div>
        </div>
        <GButton type="button" variant="danger" :disabled="revoke.isPending.value" :data-testid="`revoke-${t.id}`" @click="remove(t.id)">Revoke</GButton>
      </li>
    </ul>
    <p v-else-if="tokens.isSuccess.value" data-testid="access-token-none">No Access Tokens yet.</p>
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
code {
  overflow-wrap: anywhere;
}
.fresh {
  padding: 8px 12px;
  border: 1px solid var(--color-gold);
  border-radius: var(--radius-control);
}
.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid var(--color-rule);
}
.meta {
  color: var(--color-text-3);
  font-size: 14px;
}
.scopes {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-control);
}
</style>
