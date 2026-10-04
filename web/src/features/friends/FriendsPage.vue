<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  acceptFriendRequestMutation,
  cancelFriendRequestMutation,
  declineFriendRequestMutation,
  listFriendsOptions,
  listFriendsQueryKey,
  sendFriendRequestMutation,
  startConversationMutation,
  unblockMutation,
  unfriendMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GAvatar, GButton, GField } from '@/shared/ui'

const client = useQueryClient()
const router = useRouter()
const talk = useMutation(startConversationMutation())
function talkWith(id: string) {
  talk.mutate({ body: { with: [id] } }, { onSuccess: (c) => void router.push({ name: 'conversation', params: { conversationId: c.id } }) })
}
const page = useQuery(listFriendsOptions())
const refresh = () => client.invalidateQueries({ queryKey: listFriendsQueryKey() })
const done = { onSuccess: () => void refresh() }
const send = useMutation(sendFriendRequestMutation())
const accept = useMutation(acceptFriendRequestMutation())
const decline = useMutation(declineFriendRequestMutation())
const cancel = useMutation(cancelFriendRequestMutation())
const unfriend = useMutation(unfriendMutation())
const unblock = useMutation(unblockMutation())
const username = ref('')
const sent = ref('')
const noAccount = computed(() => (page.error.value as { status?: number } | null)?.status === 403)
const sendFailed = computed(() => {
  switch ((send.error.value as { status?: number } | null)?.status) {
    case 404:
      return 'Nobody has that Username.'
    case 409:
      return 'You are already Friends.'
    case 422:
      return 'That is you.'
    default:
      return 'The request could not be sent. Try again shortly.'
  }
})
const since = (iso: string) => new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })

function ask() {
  const name = username.value.trim()
  send.mutate({ body: { username: name } }, { onSuccess: () => {
      sent.value = name
      username.value = ''
      void refresh()
    } })
}
</script>

<template>
  <main class="g-page">
    <h1 v-if="!page.data.value">Friends</h1>
    <p v-if="noAccount" role="alert" class="g-alert" data-testid="friends-no-account">Friends need a Grimoire Account; sign in with one first.</p>
    <p v-else-if="page.isError.value" role="alert" class="g-alert">Your Friends could not be read.</p>
    <div v-else-if="page.data.value" class="g-split">
      <aside class="side">
        <h1>Friends</h1>
        <form class="stack" data-testid="friend-request-form" @submit.prevent="ask">
          <GField v-model="username" label="Add a Friend by Username" :maxlength="32" autocomplete="off" autocapitalize="none" data-testid="friend-username" />
          <p v-if="send.isError.value" role="alert" class="g-alert" data-testid="friend-request-failed">{{ sendFailed }}</p>
          <p v-else-if="sent" role="status" data-testid="friend-request-sent">Asked {{ sent }}. You become Friends once they accept.</p>
          <GButton type="submit" :disabled="!username.trim() || send.isPending.value">Send a request</GButton>
        </form>

        <section v-if="page.data.value.incoming.length" class="stack" data-testid="friend-incoming">
          <h2>Requests</h2>
          <ul class="rows">
            <li v-for="r in page.data.value.incoming" :key="r.id" :data-testid="`incoming-${r.person.username}`">
              <span><strong>{{ r.person.nickname }}</strong> <span class="dim">{{ r.person.username }}</span></span>
              <span class="actions">
                <GButton type="button" variant="primary" :data-testid="`accept-${r.person.username}`" @click="accept.mutate({ path: { requestId: r.id } }, done)">Accept</GButton>
                <GButton type="button" :data-testid="`decline-${r.person.username}`" @click="decline.mutate({ path: { requestId: r.id }, body: {} }, done)">Decline</GButton>
                <GButton type="button" variant="danger" :data-testid="`block-${r.person.username}`" @click="decline.mutate({ path: { requestId: r.id }, body: { block: true } }, done)">Decline and block</GButton>
              </span>
            </li>
          </ul>
        </section>

        <section v-if="page.data.value.outgoing.length" class="stack" data-testid="friend-outgoing">
          <h2>Waiting for an answer</h2>
          <ul class="rows">
            <li v-for="r in page.data.value.outgoing" :key="r.id">
              <span><strong>{{ r.person.nickname }}</strong> <span class="dim">{{ r.person.username }}</span></span>
              <GButton type="button" :data-testid="`cancel-${r.person.username}`" @click="cancel.mutate({ path: { requestId: r.id } }, done)">Withdraw</GButton>
            </li>
          </ul>
        </section>

        <details v-if="page.data.value.blocked.length" data-testid="friend-blocked">
          <summary>Blocked ({{ page.data.value.blocked.length }})</summary>
          <ul class="rows">
            <li v-for="b in page.data.value.blocked" :key="b.id">
              <span><strong>{{ b.person.nickname }}</strong> <span class="dim">{{ b.person.username }}</span></span>
              <GButton type="button" :data-testid="`unblock-${b.person.username}`" @click="unblock.mutate({ path: { accountId: b.person.id } }, done)">Unblock</GButton>
            </li>
          </ul>
        </details>
      </aside>
      <section class="stack main" data-testid="friend-list">
        <h2>Your Friends</h2>
        <ul v-if="page.data.value.friends.length" class="rows">
          <li v-for="f in page.data.value.friends" :key="f.person.id" :data-testid="`friend-${f.person.username}`">
            <span class="person">
              <GAvatar :name="f.person.nickname" :size="34" aria-hidden="true" />
              <span><strong>{{ f.person.nickname }}</strong> <span class="dim">@{{ f.person.username }} · since {{ since(f.since) }}</span></span>
            </span>
            <span class="actions">
              <GButton type="button" variant="primary" :data-testid="`talk-${f.person.username}`" @click="talkWith(f.person.id)">Talk</GButton>
              <GButton type="button" :data-testid="`unfriend-${f.person.username}`" @click="unfriend.mutate({ path: { accountId: f.person.id } }, done)">Remove</GButton>
            </span>
          </li>
        </ul>
        <p v-else data-testid="friend-none">No Friends yet. Ask someone by their Username.</p>
        <RouterLink :to="{ name: 'conversations' }" class="g-action">Conversations</RouterLink>
      </section>
    </div>
  </main>
</template>

<style scoped>
.side {
  display: flex;
  flex-direction: column;
  gap: 28px;
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
  margin: 0;
  padding: 0;
  list-style: none;
  border-bottom: 1px solid var(--color-rule);
}
.rows li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 0;
  border-top: 1px solid var(--color-rule);
}
.person {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.side .actions :deep(.g-button),
.side .rows :deep(.g-button) {
  min-height: 36px;
  padding: 0 12px;
  font-size: 14px;
}
.dim {
  font-size: 14px;
  color: var(--color-text-3);
}
summary {
  cursor: pointer;
  font-family: var(--font-label);
  color: var(--color-gold-high);
}
@media (pointer: coarse) {
  .side .actions :deep(.g-button),
  .side .rows :deep(.g-button) {
    min-height: 44px;
  }
}
</style>
