<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  getBuilderOptionsOptions,
  getCharacterOptions,
  listCharacterRevisionsOptions,
  listRetrainChoicesOptions,
  listRetrainsOptions,
  requestRetrainMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Ability, AbilityBase, BuildSnapshot } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { titleCase } from '@/features/compendium/highlight'
import { abbrev, abilities } from './build'

const route = useRoute()
const client = useQueryClient()
const ids = computed(() => ({ campaignId: String(route.params.id), characterId: String(route.params.characterId) }))
const sheet = useQuery(computed(() => ({ ...getCharacterOptions({ path: ids.value }), retry: false })))
const s = computed(() => sheet.data.value)
const options = useQuery(computed(() => ({ ...getBuilderOptionsOptions({ query: { ruleset: s.value?.ruleset ?? 'srd-2024', campaignId: ids.value.campaignId } }), enabled: Boolean(s.value) })))
const choices = useQuery(computed(() => ({ ...listRetrainChoicesOptions({ path: ids.value }), retry: false })))
const retrains = useQuery(computed(() => ({ ...listRetrainsOptions({ path: ids.value }), retry: false })))
const revisions = useQuery(computed(() => ({ ...listCharacterRevisionsOptions({ path: ids.value }), retry: false })))
const request = useMutation(requestRetrainMutation())

type Form = {
  species: string
  background: string
  base: AbilityBase
  bonus: Partial<Record<Ability, number>>
  increase: Partial<Record<Ability, number>>
  skills: string[]
  picks: Record<string, string>
  reason: string
}
const form = reactive<Form>({
  species: '',
  background: '',
  base: { strength: 10, dexterity: 10, constitution: 10, intelligence: 10, wisdom: 10, charisma: 10 },
  bonus: {},
  increase: {},
  skills: [],
  picks: {},
  reason: '',
})
const pickKey = (level: number, choice: string) => `${String(level)}/${choice}`
const loaded = ref(false)
watch(
  [s, () => choices.data.value],
  ([cur, cs]) => {
    if (!cur || !cs || loaded.value) return
    loaded.value = true
    form.species = cur.species.slug
    form.background = cur.background.slug
    form.base = { ...cur.base }
    form.bonus = { ...cur.bonus }
    form.increase = { ...(cur.increase ?? {}) }
    form.skills = [...cur.classSkills]
    for (const c of cs) form.picks[pickKey(c.level, c.choice)] = c.value
  },
  { immediate: true },
)
const pending = computed(() => (retrains.data.value ?? []).find((r) => r.status === 'pending'))
const skillOptions = computed(() => options.data.value?.skills.map((k) => k.skill) ?? [])
const nonZero = (m: Partial<Record<Ability, number>>) => Object.fromEntries(Object.entries(m).filter(([, v]) => v > 0))

function toggleSkill(skill: string) {
  form.skills = form.skills.includes(skill) ? form.skills.filter((k) => k !== skill) : [...form.skills, skill]
}
function submit() {
  const cur = s.value
  if (!cur) return
  const build: BuildSnapshot = {
    species: form.species,
    background: form.background,
    method: cur.method,
    base: form.base,
    bonus: nonZero(form.bonus),
    increase: nonZero(form.increase),
    skills: form.skills,
    picks: (choices.data.value ?? []).map((c) => ({ level: c.level, choice: c.choice, value: form.picks[pickKey(c.level, c.choice)] ?? c.value })),
  }
  request.mutate({ path: ids.value, body: { build, reason: form.reason } }, { onSuccess: () => void client.invalidateQueries() })
}
const statusText: Record<string, string> = { pending: 'Waiting for the DM', approved: 'Approved', declined: 'Declined' }
const when = (iso: string) => new Date(iso).toLocaleDateString()
</script>

<template>
  <main class="g-page retrain">
    <RouterLink :to="{ name: 'character', params: { id: ids.campaignId, characterId: ids.characterId } }" class="back">← Sheet</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Character</span>
      <h1>Retrain</h1>
    </header>
    <p v-if="sheet.isError.value || choices.isError.value" role="alert" class="g-alert" data-testid="retrain-error">Only the Character's player can retrain it.</p>
    <p v-else-if="!s || !options.data.value || !choices.data.value">Gathering the build…</p>
    <template v-else>
      <p class="hint">Rebuild {{ s.name }}'s choices. Your DM approves the change; the old build stays in the history.</p>
      <p v-if="pending" class="g-alert" data-testid="retrain-pending">A retrain is waiting for the DM.</p>

      <form v-else class="g-card stack" data-testid="retrain-form" @submit.prevent="submit">
        <div class="row">
          <label class="pick">
            <span>Species</span>
            <select v-model="form.species" data-testid="retrain-species">
              <option v-for="o in options.data.value.species" :key="o.slug" :value="o.slug">{{ o.name }}</option>
            </select>
          </label>
          <label class="pick">
            <span>Background</span>
            <select v-model="form.background" data-testid="retrain-background">
              <option v-for="o in options.data.value.backgrounds" :key="o.slug" :value="o.slug">{{ o.name }}</option>
            </select>
          </label>
        </div>
        <fieldset>
          <legend>Ability scores</legend>
          <div class="scores">
            <div v-for="a in abilities" :key="a" class="score">
              <span class="abbr">{{ abbrev[a] }}</span>
              <label><span class="small">Base</span><input v-model.number="form.base[a]" type="number" min="3" max="18" :data-testid="`base-${a}`" /></label>
              <label><span class="small">Origin</span><input v-model.number="form.bonus[a]" type="number" min="0" max="2" :data-testid="`bonus-${a}`" /></label>
              <label><span class="small">Improved</span><input v-model.number="form.increase[a]" type="number" min="0" max="20" :data-testid="`increase-${a}`" /></label>
            </div>
          </div>
        </fieldset>
        <fieldset>
          <legend>Class skills</legend>
          <div class="skills">
            <label v-for="k in skillOptions" :key="k" class="check">
              <input type="checkbox" :checked="form.skills.includes(k)" :data-testid="`retrain-skill-${k}`" @change="toggleSkill(k)" />
              <span>{{ titleCase(k) }}</span>
            </label>
          </div>
        </fieldset>
        <fieldset v-if="choices.data.value.length">
          <legend>Choices made on levels</legend>
          <label v-for="c in choices.data.value" :key="pickKey(c.level, c.choice)" class="pick">
            <span>Level {{ c.level }}: {{ c.name }}</span>
            <select v-model="form.picks[pickKey(c.level, c.choice)]" :data-testid="`retrain-pick-${String(c.level)}-${c.choice}`">
              <option v-for="o in c.options" :key="o.slug" :value="o.slug" :disabled="o.unmet.length > 0">
                {{ o.name }}{{ o.unmet.length ? ` (needs ${o.unmet.join(', ')})` : '' }}
              </option>
            </select>
          </label>
        </fieldset>
        <label class="pick">
          <span>Why</span>
          <textarea v-model="form.reason" maxlength="500" rows="3" data-testid="retrain-reason" />
        </label>
        <p v-if="request.error.value" role="alert" class="g-alert" data-testid="retrain-problem">{{ request.error.value.detail ?? 'That retrain was not sent.' }}</p>
        <GButton type="submit" variant="primary" :disabled="request.isPending.value" data-testid="retrain-submit">Ask the DM</GButton>
      </form>

      <section class="g-card">
        <h2>Requests</h2>
        <p v-if="!(retrains.data.value ?? []).length" class="hint">No retrains yet.</p>
        <ul class="g-list" data-testid="retrain-history">
          <li v-for="r in retrains.data.value ?? []" :key="r.id">{{ statusText[r.status] }} · {{ when(r.createdAt) }}<template v-if="r.reason"> · {{ r.reason }}</template></li>
        </ul>
        <h2>Earlier builds</h2>
        <p v-if="!(revisions.data.value ?? []).length" class="hint">No earlier builds.</p>
        <ul class="g-list" data-testid="revisions">
          <li v-for="r in revisions.data.value ?? []" :key="r.no">
            Revision {{ r.no }} · {{ titleCase(r.build.species) }} {{ titleCase(r.build.background) }} · kept by {{ r.author }} on {{ when(r.createdAt) }}
          </li>
        </ul>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.retrain {
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
  gap: 12px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
fieldset {
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 6px;
  font-family: var(--font-display);
}
.pick {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.scores {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
  gap: 8px;
}
.score {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
}
.score label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}
.abbr {
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.small {
  font-size: 12px;
  color: var(--color-text-2);
}
.skills {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 4px 12px;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 36px;
}
input[type='number'],
select,
textarea {
  min-height: 44px;
  padding: 0 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font: inherit;
}
input[type='number'] {
  width: 60px;
}
textarea {
  padding: 8px;
}
</style>
