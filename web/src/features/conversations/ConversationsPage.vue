<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { listConversationsOptions, listFriendsOptions, startConversationMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { ConversationEntry } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'

const router = useRouter()
const list = useQuery(listConversationsOptions())
const friends = useQuery(listFriendsOptions())
const start = useMutation(startConversationMutation())
const picked = ref<string[]>([])
const title = ref('')
const name = (c: ConversationEntry) => c.title || c.members.map((m) => m.nickname).join(', ')
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
const group = computed(() => picked.value.length > 1)
function open() {
  start.mutate({ body: { with: picked.value, title: group.value ? title.value.trim() : '' } }, { onSuccess: (c) => void router.push({ name: 'conversation', params: { conversationId: c.id } }) })
}
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">People</span>
      <h1>Conversations</h1>
    </header>
    <p v-if="list.isError.value" role="alert" class="g-alert">Your Conversations could not be read.</p>
    <ul v-else-if="list.data.value?.items.length" class="rows g-card" data-testid="conversation-list">
      <li v-for="c in list.data.value.items" :key="c.id">
        <RouterLink :to="{ name: 'conversation', params: { conversationId: c.id } }" class="who" :data-testid="`conversation-${c.id}`">
          <strong>{{ name(c) }}</strong>
          <span class="dim">{{ c.lastBody || 'No messages yet' }}</span>
        </RouterLink>
        <span class="meta">
          <span v-if="c.unread" class="badge" :aria-label="`${c.unread} unread`">{{ c.unread }}</span>
          <span class="dim">{{ when(c.updatedAt) }}</span>
        </span>
      </li>
    </ul>
    <p v-else-if="list.isSuccess.value" data-testid="conversation-none">No Conversations yet.</p>

    <form v-if="friends.data.value?.friends.length" class="g-card stack" data-testid="conversation-start" @submit.prevent="open">
      <h2>Talk with Friends</h2>
      <fieldset class="pick">
        <legend>Who</legend>
        <label v-for="f in friends.data.value.friends" :key="f.person.id">
          <input v-model="picked" type="checkbox" :value="f.person.id" :data-testid="`pick-${f.person.username}`" /> {{ f.person.nickname }}
        </label>
      </fieldset>
      <GField v-if="group" v-model="title" label="Group name" :maxlength="80" data-testid="conversation-title" />
      <p v-if="start.isError.value" role="alert" class="g-alert">The Conversation could not be started.</p>
      <GButton type="submit" variant="primary" :disabled="!picked.length || start.isPending.value">{{ group ? 'Start the group' : 'Open the Conversation' }}</GButton>
    </form>
    <p v-else-if="friends.isSuccess.value">Add Friends on the <RouterLink :to="{ name: 'friends' }">Friends page</RouterLink> to talk with them.</p>
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
.rows {
  display: flex;
  flex-direction: column;
  margin: 0 0 16px;
  list-style: none;
}
.rows li {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 0;
  border-bottom: 1px solid var(--color-rule);
}
.who {
  display: flex;
  flex-direction: column;
  min-width: 0;
  color: inherit;
}
.who .dim {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  display: flex;
  flex-direction: column;
  align-items: end;
  gap: 4px;
}
.badge {
  min-width: 20px;
  padding: 0 6px;
  border-radius: 999px;
  background: var(--color-gold);
  color: var(--color-ground);
  font-size: 13px;
  text-align: center;
}
.dim {
  color: var(--color-text-3);
  font-size: 14px;
}
.pick {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-control);
}
</style>
