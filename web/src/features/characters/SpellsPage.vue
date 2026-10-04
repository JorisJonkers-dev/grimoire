<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  castRitualMutation,
  copySpellMutation,
  getSpellcastingOptions,
  prepareSpellsMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { ClassSpells, GameClock, SpellPick } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const client = useQueryClient()
const ids = computed(() => ({ campaignId: String(route.params.id), characterId: String(route.params.characterId) }))
const spells = useQuery(computed(() => ({ ...getSpellcastingOptions({ path: ids.value }), retry: false })))
const sc = computed(() => spells.data.value)
const prepare = useMutation(prepareSpellsMutation())
const ritual = useMutation(castRitualMutation())
const copy = useMutation(copySpellMutation())
const drafts = reactive<Record<string, string[]>>({})
const toCopy = reactive<Record<string, string>>({})
const status = ref('')
const problem = computed(() => prepare.error.value ?? ritual.error.value ?? copy.error.value)

const refresh = () => void client.invalidateQueries()
const time = (c: GameClock) => `Day ${String(c.day)}, ${String(Math.floor(c.minute / 60)).padStart(2, '0')}:${String(c.minute % 60).padStart(2, '0')}`
const coins = computed(() => (sc.value?.purse ?? []).map((c) => `${String(c.count)} ${c.coin}`).join(', ') || 'No coins')

function draft(cs: ClassSpells): string[] {
  return drafts[cs.class] ?? cs.prepared.map((s) => s.slug)
}
function toggle(cs: ClassSpells, slug: string) {
  const now = draft(cs)
  if (now.includes(slug)) drafts[cs.class] = now.filter((s) => s !== slug)
  else if (now.length < cs.limit) drafts[cs.class] = [...now, slug]
}
function save(cs: ClassSpells) {
  status.value = ''
  prepare.mutate(
    { path: ids.value, body: { class: cs.class, spells: draft(cs) } },
    {
      onSuccess: () => {
        Reflect.deleteProperty(drafts, cs.class)
        status.value = `${cs.name} spells prepared.`
        refresh()
      },
    },
  )
}
function rituals(cs: ClassSpells): SpellPick[] {
  const seen = new Set<string>()
  return [...cs.prepared, ...cs.always, ...cs.spellbook].filter((s) => s.ritual && !seen.has(s.slug) && seen.add(s.slug))
}
function cast(spell: SpellPick) {
  status.value = ''
  ritual.mutate(
    { path: ids.value, body: { spell: spell.slug } },
    {
      onSuccess: (r) => {
        status.value = `Cast ${r.spell.name} as a ritual: ${String(r.minutes)} minutes. It is now ${time(r.clock)}.`
        refresh()
      },
    },
  )
}
function copyCost(cs: ClassSpells): string {
  const free = cs.allotment - cs.spellbook.length
  if (free > 0) return `Free: ${String(free)} of ${String(cs.allotment)} left for this level.`
  const level = cs.copyable.find((s) => s.slug === toCopy[cs.class])?.level ?? 1
  return `Costs ${String(50 * level)} gp and ${String(2 * level)} hours.`
}
function copyOne(cs: ClassSpells) {
  status.value = ''
  copy.mutate(
    { path: ids.value, body: { spell: toCopy[cs.class] ?? '' } },
    {
      onSuccess: () => {
        status.value = 'Copied into the spellbook.'
        toCopy[cs.class] = ''
        refresh()
      },
    },
  )
}
</script>

<template>
  <main class="g-page spells">
    <RouterLink :to="{ name: 'character', params: { id: ids.campaignId, characterId: ids.characterId } }" class="back">← Sheet</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Character</span>
      <h1>Spells</h1>
    </header>
    <p v-if="spells.isError.value" role="alert" class="g-alert" data-testid="spells-error">These spells could not be loaded.</p>
    <p v-else-if="!sc">Opening the spellbook…</p>
    <template v-else>
      <p class="meta" data-testid="spells-meta">{{ time(sc.clock) }} · {{ coins }}</p>
      <p v-if="sc.classes.length === 0" class="hint" data-testid="no-spells">This Character casts no spells.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="spells-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="spells-problem">{{ problem.detail ?? 'That did not work.' }}</p>

      <section v-for="cs in sc.classes" :key="cs.class" class="g-card stack" :data-testid="`spells-${cs.class}`">
        <h2>{{ cs.name }} {{ cs.level }}</h2>
        <p class="hint">Prepares {{ cs.limit }} spells, up to level {{ cs.maxLevel }}.</p>

        <div v-if="cs.cantrips.length">
          <h3>Cantrips</h3>
          <ul class="chips"><li v-for="s in cs.cantrips" :key="s.slug">{{ s.name }}</li></ul>
        </div>
        <div v-if="cs.always.length">
          <h3>Always prepared</h3>
          <ul class="chips" data-testid="always"><li v-for="s in cs.always" :key="s.slug">{{ s.name }} <small>level {{ s.level }}</small></li></ul>
        </div>

        <fieldset v-if="sc.canPrepare" :data-testid="`prepare-${cs.class}`">
          <legend>Prepared · {{ draft(cs).length }} of {{ cs.limit }}</legend>
          <p v-if="cs.options.length === 0" class="hint">{{ cs.keepsSpellbook ? 'Copy spells into your spellbook first.' : 'Nothing to prepare yet.' }}</p>
          <label v-for="s in cs.options" :key="s.slug" class="choice">
            <input type="checkbox" :checked="draft(cs).includes(s.slug)" :data-testid="`prep-${s.slug}`" @change="toggle(cs, s.slug)" />
            <span>{{ s.name }}</span><small>level {{ s.level }}{{ s.ritual ? ' · ritual' : '' }}</small>
          </label>
          <GButton variant="primary" :disabled="prepare.isPending.value" :data-testid="`prepare-save-${cs.class}`" @click="save(cs)">Prepare</GButton>
        </fieldset>
        <div v-else>
          <h3>Prepared</h3>
          <ul class="chips" data-testid="prepared"><li v-for="s in cs.prepared" :key="s.slug">{{ s.name }}</li></ul>
          <p class="hint">You change these after a long rest or a new level.</p>
        </div>

        <div v-if="rituals(cs).length">
          <h3>Rituals</h3>
          <p class="hint">Out of combat, a ritual takes 10 minutes more and uses no spell slot.</p>
          <ul class="rituals">
            <li v-for="s in rituals(cs)" :key="s.slug">
              <span>{{ s.name }}</span>
              <GButton :disabled="ritual.isPending.value" :data-testid="`ritual-${s.slug}`" @click="cast(s)">Cast as ritual</GButton>
            </li>
          </ul>
        </div>

        <div v-if="cs.keepsSpellbook" data-testid="spellbook">
          <h3>Spellbook · {{ cs.spellbook.length }} spells</h3>
          <ul class="chips"><li v-for="s in cs.spellbook" :key="s.slug">{{ s.name }}</li></ul>
          <label class="pick">
            <span>Copy a spell</span>
            <select v-model="toCopy[cs.class]" data-testid="copy-spell">
              <option value="">Choose a spell</option>
              <option v-for="s in cs.copyable" :key="s.slug" :value="s.slug">{{ s.name }} (level {{ s.level }})</option>
            </select>
          </label>
          <p class="hint" data-testid="copy-cost">{{ copyCost(cs) }}</p>
          <GButton :disabled="!toCopy[cs.class] || copy.isPending.value" data-testid="copy-submit" @click="copyOne(cs)">Copy into spellbook</GButton>
        </div>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.spells {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h1,
h2,
h3 {
  margin: 0;
  font-family: var(--font-display);
}
h3 {
  margin-bottom: 6px;
  font-size: 15px;
}
.meta,
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.chips li {
  padding: 4px 10px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-pill);
  background: var(--color-raised);
}
.chips small {
  color: var(--color-text-2);
}
fieldset {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 6px;
  font-family: var(--font-display);
}
.choice {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
  padding: 0 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
}
.choice small {
  margin-left: auto;
  color: var(--color-text-2);
}
.rituals {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.rituals li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.pick {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 8px 0 4px;
}
select {
  min-height: 44px;
  padding: 0 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font: inherit;
}
</style>
