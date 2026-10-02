<script setup lang="ts">
import { useMutation } from '@tanstack/vue-query'
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { acceptInviteMutation, previewInviteMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const token = computed(() => route.hash.replace(/^#/, ''))
const valid = computed(() => /^[A-Za-z0-9_-]{43}$/.test(token.value))
const displayName = ref('')

const preview = useMutation(previewInviteMutation())
const accept = useMutation(acceptInviteMutation())
onMounted(() => {
  if (valid.value) preview.mutate({ body: { token: token.value } })
})
function join() {
  accept.mutate(
    { body: { token: token.value, displayName: displayName.value.trim() } },
    { onSuccess: (joined) => void router.push({ name: 'campaign', params: { id: joined.id } }) },
  )
}
</script>

<template>
  <main class="g-page">
    <h1>Join a campaign</h1>
    <p v-if="!valid || preview.isError.value" role="alert" class="g-alert" data-testid="invite-invalid">
      This invite link is not valid any more. Ask your DM for a new one.
    </p>
    <p v-else-if="!preview.data.value">Reading the invitation…</p>
    <form v-else class="g-card join" data-testid="join-form" @submit.prevent="join">
      <p>
        <strong>{{ preview.data.value.invitedBy }}</strong> invites you to <strong>{{ preview.data.value.campaignName }}</strong>.
      </p>
      <GField v-model="displayName" label="Your name at this table" :maxlength="60" required data-testid="join-display-name" />
      <p v-if="accept.isError.value" role="alert" class="g-alert">You could not join. The link may have expired.</p>
      <GButton type="submit" variant="primary" :disabled="displayName.trim() === '' || accept.isPending.value">Join as Player</GButton>
    </form>
  </main>
</template>

<style scoped>
.join {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
