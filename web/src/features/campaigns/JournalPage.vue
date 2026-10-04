<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  createLoreMutation,
  createQuestMutation,
  deleteLoreMutation,
  deleteQuestMutation,
  getJournalOptions,
  readItemMutation,
  updateLoreMutation,
  updateQuestMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Lore, Quest, QuestStatus } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

// The Journal: the Quests the party has and the Lore it knows. The DM keeps both and sees what is
// still hidden or locked; a Player unlocks Lore by reading a book or letter they carry.
const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const journal = useQuery({ ...getJournalOptions(path), retry: false })
const dm = computed(() => journal.data.value?.dm ?? false)
const quests = computed(() => journal.data.value?.quests ?? [])
const lore = computed(() => journal.data.value?.lore ?? [])
const readable = computed(() => journal.data.value?.readable ?? [])

const statuses: Record<QuestStatus, string> = { hidden: 'Hidden from the Players', active: 'Active', completed: 'Completed', failed: 'Failed' }
// An item by its slug, as words: sealed-letter is the Sealed letter.
const itemName = (slug: string) => {
  const words = slug.replace(/-/g, ' ')
  return `the ${words.charAt(0).toUpperCase()}${words.slice(1)}`
}
const problem = ref('')
const learned = ref('')
const failed = () => {
  learned.value = ''
  problem.value = 'That could not be done. Check what you entered and try again.'
}
const done = () => {
  problem.value = ''
  void client.invalidateQueries()
}

const reading = useMutation(readItemMutation())
function read(itemSlug: string) {
  reading.mutate({ ...path, body: { itemSlug } }, {
    onSuccess: (r) => {
      learned.value = `You learned ${String(r.unlocked)} new Lore ${r.unlocked === 1 ? 'entry' : 'entries'}.`
      done()
    },
    onError: failed,
  })
}

// A Quest is saved whole: its words, its status and its steps.
const saveQuest = useMutation(updateQuestMutation())
function save(q: Quest, change: Partial<Pick<Quest, 'status' | 'steps'>>, onSuccess = done) {
  const body = { name: q.name, summary: q.summary, status: q.status, steps: q.steps, ...change }
  saveQuest.mutate({ path: { campaignId, questId: q.id }, body }, { onSuccess, onError: failed })
}
const tick = (q: Quest, at: number, on: boolean) => { save(q, { steps: q.steps.map((s, i) => (i === at ? { ...s, done: on } : s)) }) }
const newStep = reactive<Record<string, string>>({})
function addStep(q: Quest) {
  save(q, { steps: [...q.steps, { text: (newStep[q.id] ?? '').trim(), done: false }] }, () => {
    newStep[q.id] = ''
    done()
  })
}
const removeQuest = useMutation(deleteQuestMutation())

const quest = reactive<{ name: string; summary: string; status: QuestStatus; steps: string }>({ name: '', summary: '', status: 'hidden', steps: '' })
const createQuest = useMutation(createQuestMutation())
function addQuest() {
  const steps = quest.steps.split('\n').map((line) => line.trim()).filter(Boolean).map((text) => ({ text, done: false }))
  createQuest.mutate({ ...path, body: { name: quest.name.trim(), summary: quest.summary, status: quest.status, steps } }, {
    onSuccess: () => {
      Object.assign(quest, { name: '', summary: '', status: 'hidden', steps: '' })
      done()
    },
    onError: failed,
  })
}

// What the DM is told of a Lore entry: whether the party knows it, and what unlocks it if not.
const stateOf = (l: Lore) => (l.unlocked ? 'The party knows this.' : l.itemSlug ? `Locked. Reading ${itemName(l.itemSlug)} unlocks it.` : 'Locked.')
const saveLore = useMutation(updateLoreMutation())
function toggle(l: Lore) {
  saveLore.mutate({ path: { campaignId, loreId: l.id }, body: { title: l.title, body: l.body, itemSlug: l.itemSlug ?? '', unlocked: !l.unlocked } }, { onSuccess: done, onError: failed })
}
const removeLore = useMutation(deleteLoreMutation())

const entry = reactive({ title: '', body: '', itemSlug: '', unlocked: false })
const createLore = useMutation(createLoreMutation())
function addLore() {
  createLore.mutate({ ...path, body: { title: entry.title.trim(), body: entry.body, itemSlug: entry.itemSlug.trim(), unlocked: entry.unlocked } }, {
    onSuccess: () => {
      Object.assign(entry, { title: '', body: '', itemSlug: '', unlocked: false })
      done()
    },
    onError: failed,
  })
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Campaign</span>
      <h1>Journal</h1>
    </header>
    <p v-if="journal.isError.value" role="alert" class="g-alert" data-testid="journal-missing">That Campaign is not available.</p>
    <template v-else-if="journal.isSuccess.value">
      <p v-if="problem" role="alert" class="g-alert" data-testid="journal-problem">{{ problem }}</p>
      <p v-if="learned" role="status" class="learned" data-testid="journal-read">{{ learned }}</p>

      <section class="part" aria-labelledby="quests-title">
        <h2 id="quests-title">Quests</h2>
        <p v-if="quests.length === 0" class="hint" data-testid="no-quests">No Quests yet.</p>
        <article v-for="q in quests" :key="q.id" class="g-card entry" :aria-labelledby="`quest-title-${q.id}`" :data-testid="`quest-${q.id}`">
          <header class="row">
            <h3 :id="`quest-title-${q.id}`">{{ q.name }}</h3>
            <span :class="['status', `status--${q.status}`]" data-testid="quest-status">{{ statuses[q.status] }}</span>
          </header>
          <p v-if="q.summary" class="words" data-testid="quest-summary">{{ q.summary }}</p>
          <ol v-if="q.steps.length > 0" class="steps">
            <li v-for="(step, i) in q.steps" :key="i" data-testid="step">
              <label v-if="dm" class="check">
                <input type="checkbox" :checked="step.done" @change="tick(q, i, ($event.target as HTMLInputElement).checked)" />
                <span>{{ step.text }}</span>
              </label>
              <span v-else :class="{ struck: step.done }">{{ step.done ? 'Done' : 'To do' }}: {{ step.text }}</span>
            </li>
          </ol>
          <template v-if="dm">
            <form class="row" :aria-label="`Add a step to ${q.name}`" data-testid="step-add" @submit.prevent="addStep(q)">
              <label class="g-field grow"><span>New step</span><input v-model="newStep[q.id]" maxlength="400" /></label>
              <GButton type="submit" :disabled="(newStep[q.id] ?? '').trim() === ''">Add step</GButton>
            </form>
            <div class="row">
              <label class="g-field">
                <span>Status</span>
                <select :value="q.status" data-testid="quest-set-status" @change="save(q, { status: ($event.target as HTMLSelectElement).value as QuestStatus })">
                  <option v-for="(label, value) in statuses" :key="value" :value="value">{{ label }}</option>
                </select>
              </label>
              <GButton variant="danger" :aria-label="`Remove ${q.name}`" data-testid="quest-remove" @click="removeQuest.mutate({ path: { campaignId, questId: q.id } }, { onSuccess: done, onError: failed })">Remove</GButton>
            </div>
          </template>
        </article>
        <form v-if="dm" class="g-card entry" aria-label="Add a Quest" data-testid="quest-add" @submit.prevent="addQuest()">
          <h3>Add a Quest</h3>
          <label class="g-field"><span>Name</span><input v-model="quest.name" maxlength="120" data-testid="quest-name" /></label>
          <label class="g-field"><span>Summary</span><textarea v-model="quest.summary" maxlength="4000" rows="2" data-testid="quest-summary-new"></textarea></label>
          <label class="g-field"><span>Steps, one on each line</span><textarea v-model="quest.steps" rows="3" data-testid="quest-steps"></textarea></label>
          <label class="g-field">
            <span>Status</span>
            <select v-model="quest.status" data-testid="quest-status-new">
              <option v-for="(label, value) in statuses" :key="value" :value="value">{{ label }}</option>
            </select>
          </label>
          <GButton type="submit" variant="primary" :disabled="quest.name.trim() === ''">Add Quest</GButton>
        </form>
      </section>

      <section class="part" aria-labelledby="lore-title">
        <h2 id="lore-title">Lore</h2>
        <ul v-if="readable.length > 0" class="g-list" aria-label="Things you carry that can be read" data-testid="readable">
          <li v-for="slug in readable" :key="slug">
            <GButton :data-testid="`read-${slug}`" @click="read(slug)">Read {{ itemName(slug) }}</GButton>
          </li>
        </ul>
        <p v-if="lore.length === 0" class="hint" data-testid="no-lore">No Lore yet.</p>
        <article v-for="l in lore" :key="l.id" class="g-card entry" :aria-labelledby="`lore-title-${l.id}`" :data-testid="`lore-${l.id}`">
          <h3 :id="`lore-title-${l.id}`">{{ l.title }}</h3>
          <p v-if="dm" class="hint" data-testid="lore-state">{{ stateOf(l) }}</p>
          <p class="words" data-testid="lore-body">{{ l.body }}</p>
          <div v-if="dm" class="row">
            <GButton data-testid="lore-toggle" @click="toggle(l)">{{ l.unlocked ? 'Lock' : 'Unlock' }}</GButton>
            <GButton variant="danger" :aria-label="`Remove ${l.title}`" data-testid="lore-remove" @click="removeLore.mutate({ path: { campaignId, loreId: l.id } }, { onSuccess: done, onError: failed })">Remove</GButton>
          </div>
        </article>
        <form v-if="dm" class="g-card entry" aria-label="Add a Lore entry" data-testid="lore-add" @submit.prevent="addLore()">
          <h3>Add a Lore entry</h3>
          <label class="g-field"><span>Title</span><input v-model="entry.title" maxlength="120" data-testid="lore-title" /></label>
          <label class="g-field"><span>What it says</span><textarea v-model="entry.body" maxlength="8000" rows="4" data-testid="lore-body-new"></textarea></label>
          <label class="g-field"><span>Item that holds it, by its slug</span><input v-model="entry.itemSlug" maxlength="80" data-testid="lore-item" /></label>
          <label class="check"><input v-model="entry.unlocked" type="checkbox" data-testid="lore-unlocked" /><span>The party knows this already</span></label>
          <GButton type="submit" variant="primary" :disabled="entry.title.trim() === ''">Add Lore</GButton>
        </form>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.part,
.entry {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.row h3,
.entry h3 {
  margin: 0;
  flex: 1;
}
.grow {
  flex: 1;
  min-width: 180px;
}
.words {
  margin: 0;
  white-space: pre-wrap;
}
.steps {
  margin: 0;
  padding-left: 20px;
}
.struck {
  color: var(--color-text-2);
}
.status {
  padding: 2px 10px;
  border-radius: 999px;
  border: 1px solid var(--color-line);
  font-weight: 700;
}
.status--completed {
  border-color: var(--color-party);
}
.status--failed {
  border-color: var(--color-enemy);
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
}
.learned {
  margin: 0;
  color: var(--color-gold-high);
}
</style>
