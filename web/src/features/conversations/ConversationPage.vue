<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  listConversationsOptions,
  listMentionablesOptions,
  listMessagesOptions,
  listMessagesQueryKey,
  sendMessageMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Mentionable, MentionRef, MentionView } from '@/infrastructure/api/types.gen'
import { GAvatar, GButton, GField } from '@/shared/ui'
import ConversationRows from './ConversationRows.vue'
import { conversationName } from './names'

const route = useRoute()
const client = useQueryClient()
const path = computed(() => ({ conversationId: String(route.params.conversationId) }))
const messages = useQuery(computed(() => listMessagesOptions({ path: path.value })))
// The other Conversations stay in view beside the thread, and say what this one is called.
const others = useQuery(listConversationsOptions())
const mine = computed(() => others.data.value?.items.find((c) => c.id === path.value.conversationId))
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
    <div class="g-split">
      <aside class="side" data-testid="conversation-side">
        <h1><RouterLink :to="{ name: 'conversations' }">Conversations</RouterLink></h1>
        <ConversationRows v-if="others.data.value?.items.length" :items="others.data.value.items" :current="path.conversationId" />
        <RouterLink :to="{ name: 'conversations' }" class="g-action">New Conversation</RouterLink>
      </aside>
      <section class="main">
        <p v-if="messages.isError.value" role="alert" class="g-alert" data-testid="conversation-error">This Conversation could not be read.</p>
        <template v-else-if="messages.data.value">
          <header v-if="mine" class="who" data-testid="thread-head">
            <GAvatar :name="conversationName(mine)" :size="44" aria-hidden="true" />
            <span class="names">
              <h2>{{ conversationName(mine) }}</h2>
              <span class="dim">{{ mine.members.length }} in this Conversation</span>
            </span>
          </header>
          <ol class="thread" data-testid="thread" aria-live="polite">
            <li v-for="m in ordered" :key="m.id">
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
          <form class="stack" data-testid="composer" @submit.prevent="post">
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
      </section>
    </div>
  </main>
</template>

<style scoped>
.side {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 20px;
}
.side > :deep(.rows) {
  align-self: stretch;
}
.side h1 {
  margin: 0;
  font-size: 34px;
  overflow-wrap: anywhere;
}
@media (max-width: 899px) {
  .side h1 {
    font-size: 26px;
  }
}
.side h1 a {
  color: inherit;
  text-decoration: none;
}
.main {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.who {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--color-line);
}
.names {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.who h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 22px;
  font-weight: 600;
  letter-spacing: 0;
  text-transform: none;
  color: var(--color-text);
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
/* A thread reads like a letter: who and when in small type, then what they wrote. */
.thread {
  display: flex;
  flex-direction: column;
  gap: 22px;
  margin: 0;
  padding: 24px 0;
  list-style: none;
}
.thread li {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-width: 680px;
  margin: 0;
  padding: 0;
  border: 0;
}
.thread p {
  margin: 0;
}
.head {
  font-size: 13px;
  color: var(--color-text-2);
}
.head strong {
  color: var(--color-text);
}
.body {
  font-family: var(--font-flavour);
  font-size: 17px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
  margin: 0;
  font-size: 15px;
}
.thread .chip::before {
  content: '↳ ';
  color: var(--color-text-3);
}
form .chip {
  padding: 2px 8px;
  border: 1px solid var(--color-edge);
  border-radius: var(--radius-chip);
  font-size: 14px;
}
.chip.shut {
  color: var(--color-text-3);
}
form.stack {
  margin: 0;
  padding: 16px 0 0;
  border: 0;
  border-top: 1px solid var(--color-line);
}
.found {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}
.pick {
  width: 100%;
  min-height: 44px;
  padding: 8px 0;
  border: 0;
  border-top: 1px solid var(--color-rule);
  font: inherit;
  color: inherit;
  text-align: left;
  background: none;
  cursor: pointer;
}
.pick:hover {
  color: var(--color-gold-high);
}
.row {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.dim {
  font-size: 14px;
  color: var(--color-text-3);
}
</style>
