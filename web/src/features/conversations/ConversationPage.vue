<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  listMentionablesOptions,
  listMessagesOptions,
  listMessagesQueryKey,
  sendMessageMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Mentionable, MentionRef, MentionView } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'

const route = useRoute()
const client = useQueryClient()
const path = computed(() => ({ conversationId: String(route.params.conversationId) }))
const messages = useQuery(computed(() => listMessagesOptions({ path: path.value })))
const ordered = computed(() => [...(messages.data.value?.items ?? [])].reverse())
const send = useMutation(sendMessageMutation())
const body = ref('')
const mentions = ref<(MentionRef & { name: string })[]>([])
const search = ref('')
const picking = ref(false)
const found = useQuery(computed(() => ({ ...listMentionablesOptions({ query: { q: search.value } }), enabled: picking.value })))
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })

function link(m: MentionView) {
  if (m.kind === 'location') return { name: 'map', params: { id: m.campaignId, mapId: m.mapId ?? '' } }
  return { name: 'character', params: { id: m.campaignId, characterId: m.id } }
}
function mention(m: Mentionable) {
  if (mentions.value.length >= 10 || mentions.value.some((x) => x.id === m.id)) return
  mentions.value.push({ kind: m.kind, campaignId: m.campaignId, id: m.id, name: m.name })
  body.value = `${body.value.trimEnd()} @${m.name} `.trimStart()
  picking.value = false
  search.value = ''
}
function post() {
  send.mutate(
    { path: path.value, body: { body: body.value.trim(), mentions: mentions.value.map(({ kind, campaignId, id }) => ({ kind, campaignId, id })) } },
    { onSuccess: () => {
      body.value = ''
      mentions.value = []
      void client.invalidateQueries({ queryKey: listMessagesQueryKey({ path: path.value }) })
    } },
  )
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'conversations' }">All Conversations</RouterLink>
    <p v-if="messages.isError.value" role="alert" class="g-alert" data-testid="conversation-error">This Conversation could not be read.</p>
    <template v-else-if="messages.data.value">
      <ol class="thread" data-testid="thread" aria-live="polite">
        <li v-for="m in ordered" :key="m.id" class="g-card">
          <p class="head"><strong>{{ m.author.nickname }}</strong> <span class="dim">{{ when(m.at) }}</span></p>
          <p class="body">{{ m.body }}</p>
          <p v-if="m.mentions.length" class="chips">
            <template v-for="(x, i) in m.mentions" :key="i">
              <RouterLink v-if="x.open" :to="link(x)" class="chip" :data-testid="`mention-${x.id}`">{{ x.label }}</RouterLink>
              <span v-else class="chip shut" :data-testid="`mention-${x.id}`">Something you cannot open</span>
            </template>
          </p>
        </li>
      </ol>
      <p v-if="!ordered.length" data-testid="thread-empty">No messages yet. Say hello.</p>
      <form class="g-card stack" data-testid="composer" @submit.prevent="post">
        <GField v-model="body" label="Message" multiline :maxlength="4000" data-testid="message-body" />
        <p v-if="mentions.length" class="chips" data-testid="composer-mentions">
          <span v-for="m in mentions" :key="m.id" class="chip">{{ m.name }}</span>
        </p>
        <div v-if="picking" class="stack" data-testid="mention-picker">
          <GField v-model="search" label="Mention a Character or Location" :maxlength="60" data-testid="mention-search" />
          <ul class="found">
            <li v-for="m in found.data.value?.items ?? []" :key="m.id">
              <button type="button" class="pick" :data-testid="`pick-mention-${m.id}`" @click="mention(m)">
                {{ m.name }} <span class="dim">{{ m.kind === 'location' ? 'Location' : 'Character' }} · {{ m.campaignName }}</span>
              </button>
            </li>
          </ul>
        </div>
        <p v-if="send.isError.value" role="alert" class="g-alert" data-testid="send-failed">The message could not be sent.</p>
        <div class="row">
          <GButton type="button" data-testid="mention-open" @click="picking = !picking">{{ picking ? 'Close' : 'Mention…' }}</GButton>
          <GButton type="submit" variant="primary" :disabled="!body.trim() || send.isPending.value">Send</GButton>
        </div>
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
.thread {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 12px 0;
  padding: 0;
  list-style: none;
}
.thread p {
  margin: 0;
}
.head {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}
.body {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 6px 0 0;
}
.chip {
  padding: 2px 10px;
  border: 1px solid var(--color-gold);
  border-radius: 999px;
  font-size: 14px;
}
.chip.shut {
  border-color: var(--color-line);
  color: var(--color-text-3);
}
.found {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.pick {
  width: 100%;
  padding: 8px;
  border: 1px solid var(--color-line);
  border-radius: 8px;
  background: none;
  color: inherit;
  text-align: left;
  cursor: pointer;
}
.row {
  display: flex;
  gap: 8px;
}
.dim {
  color: var(--color-text-3);
  font-size: 14px;
}
</style>
