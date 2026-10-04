<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive } from 'vue'
import { listSharedSubmissionsOptions, reviewSharedSubmissionMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, GField } from '@/shared/ui'
import { kindNames } from './fields'

const client = useQueryClient()
const requests = useQuery(computed(() => ({ ...listSharedSubmissionsOptions(), retry: false })))
const review = useMutation(reviewSharedSubmissionMutation())
// Each pending request's IP check and notes, until it is decided.
const checks = reactive<Record<string, { clear: boolean; ipNote: string; message: string }>>({})
const check = (id: string) => (checks[id] ??= { clear: false, ipNote: '', message: '' })
const statusNames: Record<string, string> = { pending: 'Waiting', approved: 'Shared', declined: 'Declined' }

function decide(id: string, decision: 'approve' | 'decline') {
  const c = check(id)
  review.mutate(
    { path: { submissionId: id }, body: { decision, ipClear: c.clear, ipNote: c.ipNote.trim(), message: c.message.trim() } },
    { onSuccess: () => void client.invalidateQueries() },
  )
}
</script>

<template>
  <main class="g-page review">
    <RouterLink :to="{ name: 'admin' }" class="back">← Admin</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Admin</span>
      <h1>Shared Library requests</h1>
    </header>
    <p v-if="requests.isError.value" role="alert" class="g-alert" data-testid="shared-review-forbidden">Only an Admin with two-step sign-in can review the Shared Library.</p>
    <template v-else>
      <p v-if="review.error.value" role="alert" class="g-alert" data-testid="shared-review-problem">{{ review.error.value.detail ?? 'That review was not saved.' }}</p>
      <p v-if="requests.isSuccess.value && (requests.data.value ?? []).length === 0" class="hint">No requests yet.</p>
      <article v-for="r in requests.data.value ?? []" :key="r.id" class="g-card stack" :data-testid="`request-${r.name}`">
        <h2>{{ r.name }} <span class="g-tag">{{ kindNames[r.kind] }}</span> <span class="g-tag" data-testid="request-status">{{ statusNames[r.status] }}</span></h2>
        <p v-if="r.note" class="hint">{{ r.note }}</p>
        <dl class="fields">
          <template v-for="f in r.fields" :key="f.name">
            <dt>{{ f.name }}</dt>
            <dd>{{ f.value }}</dd>
          </template>
        </dl>
        <template v-if="r.status === 'pending'">
          <label class="check">
            <input v-model="check(r.id).clear" type="checkbox" data-testid="ip-clear" />
            <span>IP check: this entry carries no non-SRD text</span>
          </label>
          <GField v-model="check(r.id).ipNote" label="IP check note" :maxlength="2000" data-testid="ip-note" />
          <GField v-model="check(r.id).message" label="Message to the DM" :maxlength="2000" data-testid="review-message" />
          <div class="row">
            <GButton variant="primary" :disabled="!check(r.id).clear || review.isPending.value" data-testid="share-approve" @click="decide(r.id, 'approve')">Share it</GButton>
            <GButton :disabled="review.isPending.value" data-testid="share-decline" @click="decide(r.id, 'decline')">Decline</GButton>
          </div>
        </template>
        <p v-else class="hint" data-testid="request-outcome">
          IP check {{ r.ipClear ? 'passed' : 'not passed' }}<template v-if="r.ipNote">: {{ r.ipNote }}</template>
        </p>
      </article>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.review {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h1,
h2 {
  margin: 0;
}
.hint {
  margin: 0;
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
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.fields {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 12px;
  margin: 0;
}
dt {
  color: var(--color-text-2);
}
dd {
  margin: 0;
  overflow-wrap: anywhere;
}
</style>
