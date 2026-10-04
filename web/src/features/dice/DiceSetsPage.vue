<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import {
  chooseDiceSetMutation,
  copyDiceSetMutation,
  deleteDiceSetMutation,
  listDiceSetsOptions,
  listSharedDiceSetsOptions,
  shareDiceSetMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { DiceSet } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import DiceSetEditor from './DiceSetEditor.vue'
import DiceSheet from './DiceSheet.vue'
import { PLAIN, reviewNote } from './sets'

const client = useQueryClient()
const mine = useQuery({ ...listDiceSetsOptions(), retry: false })
const shared = useQuery({ ...listSharedDiceSetsOptions(), retry: false })
const choose = useMutation(chooseDiceSetMutation())
const share = useMutation(shareDiceSetMutation())
const copy = useMutation(copyDiceSetMutation())
const remove = useMutation(deleteDiceSetMutation())
/** The set in the editor: one of the caller's own, 'new', or none. */
const editing = ref('')
const problem = ref('')
const noAccount = computed(() => (mine.error.value as { status?: number } | null)?.status === 403)
const sets = computed(() => mine.data.value?.items ?? [])
const edited = computed(() => sets.value.find((s) => s.id === editing.value))
const sharings = { private: 'Only you', friends: 'Your Friends', everyone: 'Everyone' }
const face = (s: DiceSet) => s.design.dice.d20 ?? PLAIN

function after(what: string) {
  return {
    onSuccess: () => { problem.value = ''; void client.invalidateQueries() },
    onError: (e: unknown) => {
      problem.value = (e as { status?: number } | null)?.status === 409 && what === 'copied' ? 'You already have a copy of that set.' : `The set could not be ${what}. Try again shortly.`
    },
  }
}
const rollWith = (id?: string) => { choose.mutate({ body: id ? { diceSetId: id } : {} }, after('chosen')) }
function drop(s: DiceSet) {
  if (editing.value === s.id) editing.value = ''
  remove.mutate({ path: { diceSetId: s.id } }, after('deleted'))
}
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">Your dice</span>
      <h1>Dice Sets</h1>
    </header>
    <p v-if="noAccount" role="alert" class="g-alert" data-testid="dice-sets-no-account">Dice Sets need a Grimoire Account; sign in with one first.</p>
    <p v-else-if="mine.isError.value" role="alert" class="g-alert">Your Dice Sets could not be read.</p>
    <template v-else-if="mine.data.value">
      <p v-if="problem" role="alert" class="g-alert" data-testid="dice-sets-problem">{{ problem }}</p>
      <section class="g-card stack" data-testid="dice-sets-mine">
        <h2>Your dice</h2>
        <ul class="rows">
          <li data-testid="dice-set-plain">
            <label class="pick">
              <input type="radio" name="dice-set" :checked="!mine.data.value.chosen" data-testid="dice-choice-plain" @change="rollWith()" />
              <span><strong>The plain dice</strong></span>
            </label>
          </li>
          <li v-for="s in sets" :key="s.id" :data-testid="`dice-set-${s.name}`">
            <label class="pick">
              <input type="radio" name="dice-set" :checked="mine.data.value.chosen === s.id" :data-testid="`dice-choice-${s.name}`" @change="rollWith(s.id)" />
              <DiceSheet type="d20" :look="face(s)" :image-url="s.imageUrl" :size="56" />
              <span>
                <strong>{{ s.name }}</strong>
                <span v-if="s.copy" class="dim" :data-testid="`copy-of-${s.name}`"> a copy, by {{ s.by }}</span>
              </span>
            </label>
            <span class="actions">
              <label v-if="!s.copy" class="g-field share">
                <span>Shared with</span>
                <select :value="s.sharing" :data-testid="`sharing-${s.name}`" @change="share.mutate({ path: { diceSetId: s.id }, body: { sharing: ($event.target as HTMLSelectElement).value as DiceSet['sharing'] } }, after('shared'))">
                  <option v-for="(text, value) in sharings" :key="value" :value="value">{{ text }}</option>
                </select>
              </label>
              <GButton v-if="!s.copy" type="button" :data-testid="`edit-${s.name}`" @click="editing = s.id">Edit</GButton>
              <GButton type="button" variant="danger" :data-testid="`delete-${s.name}`" @click="drop(s)">Delete</GButton>
            </span>
            <p v-if="reviewNote(s)" class="note" role="status" :data-testid="`review-${s.name}`">{{ reviewNote(s) }}</p>
          </li>
        </ul>
        <GButton v-if="editing !== 'new'" type="button" variant="primary" data-testid="dice-set-new" @click="editing = 'new'">Design a Dice Set</GButton>
      </section>

      <DiceSetEditor v-if="editing === 'new' || edited" :key="editing" :set="edited" @saved="editing = $event" @done="editing = ''" />

      <section class="g-card stack" data-testid="dice-sets-shared">
        <h2>Shared with you</h2>
        <ul v-if="shared.data.value?.items.length" class="rows">
          <li v-for="s in shared.data.value.items" :key="s.id" :data-testid="`shared-set-${s.name}`">
            <span class="pick">
              <DiceSheet type="d20" :look="face(s)" :image-url="s.imageUrl" :size="56" />
              <span><strong>{{ s.name }}</strong> <span class="dim">by {{ s.by }}</span></span>
            </span>
            <GButton type="button" :data-testid="`copy-${s.name}`" @click="copy.mutate({ path: { diceSetId: s.id } }, after('copied'))">Take a copy</GButton>
          </li>
        </ul>
        <p v-else data-testid="shared-none">Nothing yet. Sets your Friends share, and sets shared with everyone, show here.</p>
      </section>
    </template>
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
  margin: 0;
  padding: 0;
  list-style: none;
}
.rows li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 0;
  border-bottom: 1px solid var(--color-rule);
}
.pick {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
}
.pick input {
  width: 22px;
  height: 22px;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 8px;
}
.share select {
  min-height: 44px;
}
.note {
  flex-basis: 100%;
  margin: 0;
  color: var(--color-text-2);
}
.dim {
  color: var(--color-text-2);
}
</style>
