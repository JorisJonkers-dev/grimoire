<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  listNotificationsOptions,
  listNotificationsQueryKey,
  readAllNotificationsMutation,
  readNotificationMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { NotificationEntry } from '@/infrastructure/api/types.gen'

const router = useRouter()
const client = useQueryClient()
const list = useQuery({ ...listNotificationsOptions(), refetchInterval: 60_000 })
const refresh = () => client.invalidateQueries({ queryKey: listNotificationsQueryKey() })
const readOne = useMutation(readNotificationMutation())
const readAll = useMutation(readAllNotificationsMutation())
const open = ref(false)
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })

function act(n: NotificationEntry) {
  open.value = false
  if (!n.read) readOne.mutate({ path: { notificationId: n.id } }, { onSuccess: () => void refresh() })
  void router.push(n.actionPath)
}
</script>

<template>
  <div class="bell">
    <button
      type="button"
      class="toggle"
      :aria-expanded="open"
      aria-controls="notification-panel"
      :aria-label="list.data.value?.unread ? `Notifications, ${list.data.value.unread} unread` : 'Notifications'"
      data-testid="bell"
      @click="open = !open"
    >
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3a6 6 0 0 0-6 6v4l-2 3h16l-2-3V9a6 6 0 0 0-6-6zm-2 15a2 2 0 0 0 4 0" fill="none" stroke="currentColor" stroke-width="1.8" /></svg>
      <span v-if="list.data.value?.unread" class="count" data-testid="bell-count">{{ list.data.value.unread }}</span>
    </button>
    <section v-if="open" id="notification-panel" class="panel" data-testid="bell-panel" aria-label="Notifications">
      <div class="head">
        <strong>Notifications</strong>
        <button v-if="list.data.value?.unread" type="button" class="link" data-testid="bell-read-all" @click="readAll.mutate({}, { onSuccess: () => void refresh() })">Mark all read</button>
      </div>
      <ul v-if="list.data.value?.items.length">
        <li v-for="n in list.data.value.items" :key="n.id" :class="{ unread: !n.read }" :data-testid="`notification-${n.id}`">
          <p class="title">{{ n.title }}</p>
          <p v-if="n.body" class="body">{{ n.body }}</p>
          <p class="foot">
            <span class="dim">{{ when(n.at) }}</span>
            <button type="button" class="action" :data-testid="`act-${n.id}`" @click="act(n)">{{ n.actionLabel }}</button>
          </p>
        </li>
      </ul>
      <p v-else class="dim" data-testid="bell-empty">Nothing new.</p>
    </section>
  </div>
</template>

<style scoped>
.bell {
  position: relative;
}
.toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--color-bar-text);
  cursor: pointer;
}
.toggle[aria-expanded='true'],
.toggle:hover {
  color: var(--color-bar-brand);
}
.toggle svg {
  width: 21px;
  height: 21px;
}
.count {
  position: absolute;
  top: 4px;
  right: 2px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  box-sizing: border-box;
  border-radius: 8px;
  background: var(--color-badge);
  color: var(--color-badge-text);
  font-size: 10px;
  font-weight: 800;
  line-height: 16px;
  text-align: center;
}
.panel {
  position: absolute;
  z-index: 30;
  top: 48px;
  right: 0;
  width: min(380px, calc(100vw - 2 * var(--gutter)));
  max-height: 70vh;
  overflow-y: auto;
  padding: 14px 16px;
  border: 1px solid var(--color-edge);
  border-radius: var(--radius-panel);
  background: var(--color-surface);
  box-shadow: 0 14px 32px rgb(0 0 0 / 55%);
  color: var(--color-text);
}
.head {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
}
ul {
  margin: 0;
  padding: 0;
  list-style: none;
}
li {
  padding: 8px 0;
  border-top: 1px solid var(--color-line);
}
li.unread .title {
  color: var(--color-gold-high);
}
p {
  margin: 0;
}
.body {
  color: var(--color-text-2);
  font-size: 14px;
  overflow-wrap: anywhere;
}
.foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 4px;
}
.action,
.link {
  border: 0;
  background: none;
  color: var(--color-gold);
  cursor: pointer;
  text-decoration: underline;
}
.dim {
  color: var(--color-text-3);
  font-size: 13px;
}
</style>
