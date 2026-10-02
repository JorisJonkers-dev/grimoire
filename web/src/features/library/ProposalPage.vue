<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  getCampaignOptions,
  getProposalOptions,
  resubmitProposalMutation,
  reviewProposalMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LibraryField, ProposalDetail } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'
import FieldsEditor from './FieldsEditor.vue'
import { cleanFields, compare, copyFields, kindNames, statusNames, stepNames } from './fields'

const route = useRoute()
const client = useQueryClient()
const path = computed(() => ({ path: { campaignId: String(route.params.id), proposalId: String(route.params.proposalId) } }))
const detail = useQuery(computed(() => ({ ...getProposalOptions(path.value), retry: false })))
const campaign = useQuery(computed(() => ({ ...getCampaignOptions({ path: { campaignId: String(route.params.id) } }), retry: false })))
const dm = computed(() => campaign.data.value?.myRole === 'dm')
const d = computed(() => detail.data.value)
const review = useMutation(reviewProposalMutation())
const resubmit = useMutation(resubmitProposalMutation())
const problem = computed(() => review.error.value ?? resubmit.error.value)
const name = ref('')
const fields = ref<LibraryField[]>([])
const message = ref('')
watch(
  d,
  (cur) => {
    if (!cur) return
    name.value = cur.proposal.name
    fields.value = copyFields(cur.proposal.fields)
  },
  { immediate: true },
)
const rows = computed(() => compare(d.value?.current?.fields ?? [], d.value?.proposal.fields ?? []))
const when = (iso: string) => new Date(iso).toLocaleDateString()
const settle = (next: ProposalDetail) => {
  client.setQueryData(getProposalOptions(path.value).queryKey, next)
  message.value = ''
  void client.invalidateQueries()
}

function decide(action: 'approve' | 'request_changes' | 'decline') {
  const body = action === 'approve'
    ? { action, message: message.value.trim(), name: name.value.trim(), fields: cleanFields(fields.value) }
    : { action, message: message.value.trim() }
  review.mutate({ ...path.value, body }, { onSuccess: settle })
}
function sendAgain() {
  resubmit.mutate({ ...path.value, body: { name: name.value.trim(), fields: cleanFields(fields.value), note: message.value.trim() } }, { onSuccess: settle })
}
</script>

<template>
  <main class="g-page proposal">
    <RouterLink :to="{ name: 'proposals', params: { id: String(route.params.id) } }" class="back">← Proposals</RouterLink>
    <p v-if="detail.isError.value" role="alert" class="g-alert" data-testid="proposal-error">There is no such Proposal for you here.</p>
    <p v-else-if="!d">Opening the Proposal…</p>
    <template v-else>
      <h1>{{ d.proposal.name }} <span class="g-tag">{{ kindNames[d.proposal.kind] }}</span></h1>
      <p class="hint">From {{ d.proposal.authorName }} · <span :class="['g-tag', d.proposal.status]" data-testid="proposal-status">{{ statusNames[d.proposal.status] }}</span></p>
      <p v-if="d.proposal.message" class="g-card said" data-testid="proposal-message"><strong>The DM:</strong> {{ d.proposal.message }}</p>
      <p v-if="d.proposal.note" class="said" data-testid="proposal-note-shown"><strong>{{ d.proposal.authorName }}:</strong> {{ d.proposal.note }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="proposal-problem">{{ problem.detail ?? 'That did not work.' }}</p>

      <section class="g-card diff" aria-label="Side by side" data-testid="proposal-diff">
        <h2>{{ d.current ? 'Now' : 'A new entry' }}</h2>
        <h2>Proposed</h2>
        <template v-for="r in rows" :key="r.name">
          <div :class="['cell', { changed: r.changed }]" :data-testid="`now-${r.name}`">
            <span class="name">{{ r.name }}</span> {{ r.now ?? '—' }}
          </div>
          <div :class="['cell', { changed: r.changed }]" :data-testid="`proposed-${r.name}`">
            <span class="name">{{ r.name }}</span> {{ r.proposed ?? '—' }}
          </div>
        </template>
      </section>

      <form v-if="dm && d.proposal.status === 'pending'" class="g-card stack" data-testid="proposal-review" @submit.prevent="decide('approve')">
        <h2>Review</h2>
        <GField v-model="name" label="Name" :maxlength="80" data-testid="review-name" />
        <FieldsEditor v-model="fields" label="Fields to approve" />
        <GField v-model="message" label="Message to the player" :maxlength="2000" multiline data-testid="review-message" />
        <div class="row">
          <GButton type="submit" variant="primary" :disabled="review.isPending.value" data-testid="approve">Approve</GButton>
          <GButton :disabled="review.isPending.value || message.trim() === ''" data-testid="request-changes" @click="decide('request_changes')">Ask for changes</GButton>
          <GButton :disabled="review.isPending.value" data-testid="decline" @click="decide('decline')">Decline</GButton>
        </div>
      </form>
      <form v-else-if="!dm && d.proposal.status === 'changes_requested'" class="g-card stack" data-testid="proposal-resubmit" @submit.prevent="sendAgain">
        <h2>Change it and send it again</h2>
        <GField v-model="name" label="Name" :maxlength="80" data-testid="resubmit-name" />
        <FieldsEditor v-model="fields" label="Fields" />
        <GField v-model="message" label="Note to the DM" :maxlength="2000" multiline data-testid="resubmit-note" />
        <GButton type="submit" variant="primary" :disabled="resubmit.isPending.value || name.trim() === ''" data-testid="resubmit">Send again</GButton>
      </form>

      <section class="g-card stack" data-testid="proposal-steps">
        <h2>History</h2>
        <ol class="g-list">
          <li v-for="s in d.steps" :key="s.no">{{ stepNames[s.action] }} by {{ s.by }} · {{ when(s.createdAt) }}<template v-if="s.message"> · {{ s.message }}</template></li>
        </ol>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.proposal {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h1,
h2 {
  margin: 0;
  font-family: var(--font-display);
}
h2 {
  font-size: 17px;
}
.hint,
.said {
  margin: 0;
}
.hint {
  color: var(--color-text-2);
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.diff {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px 12px;
}
.cell {
  min-width: 0;
  padding: 4px 6px;
  border-radius: var(--radius-md);
  overflow-wrap: anywhere;
}
.cell.changed {
  background: var(--color-raised);
  box-shadow: inset 3px 0 0 var(--color-gold-high);
}
.name {
  display: block;
  font-size: 12px;
  color: var(--color-text-2);
}
</style>
