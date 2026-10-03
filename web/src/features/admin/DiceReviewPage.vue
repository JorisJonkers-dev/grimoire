<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { listDiceSetsToReviewOptions, reviewDiceSetMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import DiceSheet from '@/features/dice/DiceSheet.vue'
import { PLAIN } from '@/features/dice/sets'
import { GButton } from '@/shared/ui'

// A Dice Set with an uploaded picture waits here before everyone can see it.
const client = useQueryClient()
const waiting = useQuery({ ...listDiceSetsToReviewOptions(), retry: false })
const review = useMutation(reviewDiceSetMutation())
const decide = (id: string, approve: boolean) => { review.mutate({ path: { diceSetId: id }, body: { approve } }, { onSuccess: () => void client.invalidateQueries() }) }
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'admin' }" class="back">← Admin</RouterLink>
    <h1>Dice Set pictures</h1>
    <p v-if="waiting.isError.value" role="alert" class="g-alert" data-testid="dice-review-forbidden">Only an Admin with two-step sign-in can check Dice Sets.</p>
    <template v-else-if="waiting.data.value">
      <p v-if="review.isError.value" role="alert" class="g-alert" data-testid="dice-review-problem">That decision was not saved. Try again shortly.</p>
      <p v-if="waiting.data.value.items.length === 0" data-testid="dice-review-none">No pictures wait to be checked.</p>
      <article v-for="s in waiting.data.value.items" :key="s.id" class="g-card stack" :data-testid="`review-set-${s.name}`">
        <h2>{{ s.name }} <span class="dim">by {{ s.by }}</span></h2>
        <div class="look">
          <img :src="s.imageUrl" :alt="`The picture on ${s.name}`" class="picture" data-testid="review-picture" />
          <DiceSheet type="d20" :look="s.design.dice.d20 ?? PLAIN" :image-url="s.imageUrl" :size="160" />
        </div>
        <div class="actions">
          <GButton type="button" variant="primary" data-testid="review-approve" @click="decide(s.id, true)">Approve</GButton>
          <GButton type="button" variant="danger" data-testid="review-reject" @click="decide(s.id, false)">Turn down</GButton>
        </div>
      </article>
    </template>
  </main>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 12px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.look,
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.picture {
  max-width: min(100%, 320px);
  max-height: 320px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
}
.dim {
  color: var(--color-text-2);
}
</style>
