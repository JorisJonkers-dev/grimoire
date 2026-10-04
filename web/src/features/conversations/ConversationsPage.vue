<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { listConversationsOptions, listFriendsOptions, startConversationMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'
import ConversationRows from './ConversationRows.vue'

const router = useRouter()
const list = useQuery(listConversationsOptions())
const friends = useQuery(listFriendsOptions())
const start = useMutation(startConversationMutation())
const picked = ref<string[]>([])
const title = ref('')
const group = computed(() => picked.value.length > 1)
function open() {
  start.mutate({ body: { with: picked.value, title: group.value ? title.value.trim() : '' } }, { onSuccess: (c) => void router.push({ name: 'conversation', params: { conversationId: c.id } }) })
}
</script>

<template>
  <main class="g-page">
    <div class="g-split">
      <aside class="side">
        <h1>Conversations</h1>
        <p v-if="list.isError.value" role="alert" class="g-alert">Your Conversations could not be read.</p>
        <ConversationRows v-else-if="list.data.value?.items.length" :items="list.data.value.items" />
        <p v-else-if="list.isSuccess.value" data-testid="conversation-none">No Conversations yet.</p>
        <RouterLink :to="{ name: 'friends' }" class="g-action">Friends</RouterLink>
      </aside>
      <section class="main">
        <form v-if="friends.data.value?.friends.length" class="stack" data-testid="conversation-start" @submit.prevent="open">
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
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 520px;
}
h2 {
  margin: 0;
}
.pick {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  border: 0;
}
.pick legend {
  padding: 0 0 4px;
  font-size: 14px;
  color: var(--color-text-3);
}
.pick label {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
  border-top: 1px solid var(--color-rule);
}
</style>
