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
import CampaignFrame from '@/features/campaigns/CampaignFrame.vue'
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
    <CampaignFrame current="proposals">
      <header class="g-headline">
        <span class="g-eyebrow">Proposal<template v-if="d"> · {{ kindNames[d.proposal.kind] }}</template></span>
        <h1>{{ d?.proposal.name ?? 'Proposal' }}</h1>
        <p v-if="d" class="g-meta">
          <span>From {{ d.proposal.authorName }}</span>
          <span :class="['state', d.proposal.status]" data-testid="proposal-status">{{ statusNames[d.proposal.status] }}</span>
        </p>
      </header>
    </CampaignFrame>
    <p v-if="detail.isError.value" role="alert" class="g-alert" data-testid="proposal-error">There is no such Proposal for you here.</p>
    <p v-else-if="!d">Opening the Proposal…</p>
    <div v-else class="cols">
      <div class="main">
        <p v-if="problem" role="alert" class="g-alert" data-testid="proposal-problem">{{ problem.detail ?? 'That did not work.' }}</p>

        <section class="part" aria-label="Side by side">
          <h2>What changes</h2>
          <div class="diff" data-testid="proposal-diff">
            <h3>{{ d.current ? 'Now' : 'A new entry' }}</h3>
            <h3>Proposed</h3>
            <template v-for="r in rows" :key="r.name">
              <div :class="['cell', { changed: r.changed }]" :data-testid="`now-${r.name}`">
                <span class="name">{{ r.name }}</span> {{ r.now ?? '—' }}
              </div>
              <div :class="['cell', { changed: r.changed }]" :data-testid="`proposed-${r.name}`">
                <span class="name">{{ r.name }}</span> {{ r.proposed ?? '—' }}
              </div>
            </template>
          </div>
        </section>

        <section v-if="d.proposal.message || d.proposal.note" class="part">
          <h2>Conversation</h2>
          <p v-if="d.proposal.note" class="said" data-testid="proposal-note-shown"><strong>{{ d.proposal.authorName }}:</strong> {{ d.proposal.note }}</p>
          <p v-if="d.proposal.message" class="said" data-testid="proposal-message"><strong>The DM:</strong> {{ d.proposal.message }}</p>
        </section>

        <section class="part" data-testid="proposal-steps">
          <h2>History</h2>
          <ol class="g-list">
            <li v-for="s in d.steps" :key="s.no">{{ stepNames[s.action] }} by {{ s.by }} · {{ when(s.createdAt) }}<template v-if="s.message"> · {{ s.message }}</template></li>
          </ol>
        </section>
      </div>

      <aside class="side">
        <form v-if="dm && d.proposal.status === 'pending'" class="stack" data-testid="proposal-review" @submit.prevent="decide('approve')">
          <h2>Decision</h2>
          <GField v-model="name" label="Name" :maxlength="80" data-testid="review-name" />
          <FieldsEditor v-model="fields" label="Fields to approve" />
          <GField v-model="message" label="Message to the player" :maxlength="2000" multiline data-testid="review-message" />
          <div class="row">
            <GButton type="submit" variant="primary" :disabled="review.isPending.value" data-testid="approve">Approve</GButton>
            <GButton :disabled="review.isPending.value || message.trim() === ''" data-testid="request-changes" @click="decide('request_changes')">Ask for changes</GButton>
            <GButton :disabled="review.isPending.value" data-testid="decline" @click="decide('decline')">Decline</GButton>
          </div>
        </form>
        <form v-else-if="!dm && d.proposal.status === 'changes_requested'" class="stack" data-testid="proposal-resubmit" @submit.prevent="sendAgain">
          <h2>Change it and send it again</h2>
          <GField v-model="name" label="Name" :maxlength="80" data-testid="resubmit-name" />
          <FieldsEditor v-model="fields" label="Fields" />
          <GField v-model="message" label="Note to the DM" :maxlength="2000" multiline data-testid="resubmit-note" />
          <GButton type="submit" variant="primary" :disabled="resubmit.isPending.value || name.trim() === ''" data-testid="resubmit">Send again</GButton>
        </form>

        <p v-else class="hint">{{ dm ? 'Nothing is waiting for you here.' : 'Your DM decides this one.' }}</p>
      </aside>
    </div>
  </main>
</template>

<style scoped>
.proposal {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
h1,
h2,
h3 {
  margin: 0;
}
.state {
  color: var(--color-gold-high);
}
.state.approved {
  color: var(--color-success);
}
.state.declined {
  color: var(--color-danger-text);
}
.cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 36px 56px;
  align-items: start;
}
.main {
  display: flex;
  flex-direction: column;
  gap: 40px;
  min-width: 0;
}
.part,
.side,
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.hint,
.said {
  margin: 0;
}
.hint {
  font-size: 14px;
  color: var(--color-text-3);
}
.said {
  font-family: var(--font-flavour);
  font-size: 16px;
  line-height: 1.5;
}
.said strong {
  font-family: var(--font-ui);
  font-size: 13px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
/* What is there now beside what is proposed, a row to each field; a change is marked down its edge. */
.diff {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 16px;
  font-size: 15px;
}
.diff h3 {
  padding-bottom: 10px;
  font-family: var(--font-label);
  font-size: 14px;
  font-weight: 400;
  color: var(--color-text-3);
}
.cell {
  min-width: 0;
  padding: 12px 0 12px 10px;
  border-top: 1px solid var(--color-rule);
  overflow-wrap: anywhere;
}
.cell.changed {
  box-shadow: inset 2px 0 0 var(--color-gold);
  background: var(--color-surface);
}
.name {
  display: block;
  font-size: 13px;
  color: var(--color-text-3);
}
@media (max-width: 899px) {
  .cols {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
