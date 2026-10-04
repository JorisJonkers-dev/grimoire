<script setup lang="ts">
import type { ConversationEntry } from '@/infrastructure/api/types.gen'
import { GAvatar } from '@/shared/ui'
import { conversationName } from './names'

// Conversations as rows: a face, who it is with, the last thing said, and what is unread.
defineProps<{ items: ConversationEntry[]; current?: string }>()
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
</script>

<template>
  <ul class="rows" data-testid="conversation-list">
    <li v-for="c in items" :key="c.id">
      <RouterLink
        :to="{ name: 'conversation', params: { conversationId: c.id } }"
        :class="['row', { on: c.id === current }]"
        :aria-current="c.id === current ? 'page' : undefined"
        :data-testid="`conversation-${c.id}`"
      >
        <span v-if="c.members.length > 2" class="face group" aria-hidden="true">{{ c.members.length }}</span>
        <GAvatar v-else class="face" :name="conversationName(c)" :size="34" aria-hidden="true" />
        <span class="text">
          <span class="top">
            <strong>{{ conversationName(c) }}</strong>
            <span v-if="c.unread" class="new" :aria-label="`${c.unread} unread`">{{ c.unread }}</span>
            <span v-else class="dim">{{ when(c.updatedAt) }}</span>
          </span>
          <span class="last">{{ c.lastBody || 'No messages yet' }}</span>
        </span>
      </RouterLink>
    </li>
  </ul>
</template>

<style scoped>
.rows {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
  border-bottom: 1px solid var(--color-rule);
}
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0 12px 10px;
  border-top: 1px solid var(--color-rule);
  border-left: 2px solid transparent;
  color: var(--color-text);
  text-decoration: none;
}
.row:hover strong {
  color: var(--color-gold-high);
}
.row.on {
  border-left-color: var(--color-gold);
  background: var(--color-surface);
}
.face {
  flex: none;
}
.group {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  width: 34px;
  height: 34px;
  border: 1px solid var(--color-edge);
  border-radius: 50%;
  font-family: var(--font-display);
  font-size: 12px;
  font-weight: 700;
  color: var(--color-text-2);
}
.text {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}
.top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}
.top strong {
  overflow: hidden;
  font-size: 15px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dim {
  flex: none;
  font-size: 12px;
  color: var(--color-text-3);
}
.new {
  flex: none;
  min-width: 18px;
  padding: 0 5px;
  border-radius: 9px;
  font-size: 12px;
  font-weight: 800;
  line-height: 18px;
  text-align: center;
  color: var(--color-badge-text);
  background: var(--color-badge);
}
.last {
  overflow: hidden;
  font-size: 14px;
  color: var(--color-text-2);
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
