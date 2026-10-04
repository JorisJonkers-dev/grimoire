<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createCharacterMutation,
  discardCharacterDraftMutation,
  getBuilderOptionsOptions,
  getCampaignOptions,
  previewCharacterMutation,
  rollCharacterScoresMutation,
  saveCharacterDraftMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import { getCharacterDraft } from '@/infrastructure/api/sdk.gen'
import type { Ability, AbilityBase, CharacterBuild, CharacterDraft, CharacterDraftBuild } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'
import { titleCase } from '@/features/compendium/highlight'
import {
  abbrev,
  abilities,
  baseValid,
  bonusFor,
  bonusValid,
  type BonusPattern,
  defaultBase,
  type Method,
  modifier,
  pointsSpent,
  signed,
  standardArray,
} from './build'

const route = useRoute()
const router = useRouter()
const id = computed(() => String(route.params.id))
const campaign = useQuery(computed(() => ({ ...getCampaignOptions({ path: { campaignId: id.value } }), retry: false })))
const ruleset = computed(() => campaign.data.value?.ruleset)
const options = useQuery(
  computed(() => ({
    ...getBuilderOptionsOptions({ query: { ruleset: ruleset.value ?? 'srd-2024', campaignId: id.value } }),
    enabled: ruleset.value !== undefined,
  })),
)
const o = computed(() => options.data.value)
const methods = computed<Method[]>(() => (campaign.data.value?.creationMethods ?? ['standard-array', 'point-buy', 'rolled']))
const startingLevel = computed(() => campaign.data.value?.startingLevel ?? 1)
const methodNames: Record<Method, string> = { 'standard-array': 'Standard array', 'point-buy': 'Point buy', rolled: 'Rolled (4d6, drop lowest)' }

const steps = ['Species', 'Class', 'Background', 'Ability scores', 'Skills', 'Equipment', 'Appearance', 'Name and story', 'Review'] as const
const step = ref(0)
const name = ref('')
const species = ref('')
const background = ref('')
const klass = ref('')
const method = ref<Method>('standard-array')
const base = reactive<AbilityBase>(defaultBase('standard-array'))
const pattern = ref<BonusPattern>('two-one')
const picks = ref<Ability[]>([])
const skills = ref<string[]>([])
const armor = ref('')
const shield = ref(false)
const weapons = ref<string[]>([])
const appearance = ref('')
const backstory = ref('')
const rolled = ref<number[]>([])
// placed maps each ability to the index of the rolled score put on it.
const placed = reactive<Partial<Record<Ability, number>>>({})
const restoring = ref(false)

watch(method, (m) => {
  if (!restoring.value) Object.assign(base, defaultBase(m))
})
watch(background, () => {
  if (!restoring.value) picks.value = []
})
watch(klass, () => {
  if (!restoring.value) skills.value = []
})
watch(methods, (allowed) => {
  if (!allowed.includes(method.value) && allowed[0]) method.value = allowed[0]
}, { immediate: true })

// What has been chosen so far, shown under its step.
const chosen = computed(() => [
  o.value?.species.find((x) => x.slug === species.value)?.name,
  o.value?.classes.find((x) => x.slug === klass.value)?.name,
  o.value?.backgrounds.find((x) => x.slug === background.value)?.name,
  undefined, undefined, undefined, undefined,
  name.value.trim() || undefined,
  undefined,
])
const chosenBackground = computed(() => o.value?.backgrounds.find((b) => b.slug === background.value))
const chosenClass = computed(() => o.value?.classes.find((c) => c.slug === klass.value))
const bonusChoices = computed<readonly Ability[]>(() => {
  const listed = chosenBackground.value?.abilities ?? []
  return listed.length > 0 ? listed : abilities
})
const bonus = computed(() => bonusFor(pattern.value, picks.value))
const slots = computed(() => (pattern.value === 'two-one' ? ['+2', '+1'] : ['+1', '+1', '+1']))
const skillOptions = computed(() => (o.value?.skills ?? []).filter((s) => !(chosenBackground.value?.skills ?? []).includes(s.skill)))
const skillLimit = computed(() => chosenClass.value?.skillChoices ?? 2)
const budget = computed(() => o.value?.pointBuyBudget ?? 27)
const allPlaced = computed(() => {
  const used = abilities.map((a) => placed[a])
  return rolled.value.length === 6 && used.every((i) => i !== undefined) && new Set(used).size === 6
})

const stepValid = computed(() => {
  switch (step.value) {
    case 0:
      return species.value !== ''
    case 1:
      return klass.value !== ''
    case 2:
      return background.value !== ''
    case 3:
      return (method.value !== 'rolled' || allPlaced.value) && baseValid(method.value, base, budget.value) && bonusValid(bonus.value)
    case 4:
      return skills.value.length === skillLimit.value
    case 5:
      return weapons.value.length <= 4
    case 7:
      return name.value.trim() !== ''
    default:
      return true
  }
})

const build = computed<CharacterBuild>(() => ({
  name: name.value.trim(),
  species: species.value,
  class: klass.value,
  background: background.value,
  method: method.value,
  base: { ...base },
  bonus: bonus.value,
  skills: skills.value,
  armor: armor.value,
  shield: shield.value,
  weapons: weapons.value,
  appearance: appearance.value.trim(),
  backstory: backstory.value.trim(),
}))

// draftBuild is what the wizard keeps between visits; empty choices stay out.
const draftBuild = computed<CharacterDraftBuild>(() => {
  const b: CharacterDraftBuild = { method: method.value, base: { ...base }, bonus: bonus.value, skills: skills.value, shield: shield.value, weapons: weapons.value }
  if (name.value) b.name = name.value
  if (species.value) b.species = species.value
  if (klass.value) b.class = klass.value
  if (background.value) b.background = background.value
  if (armor.value) b.armor = armor.value
  if (appearance.value) b.appearance = appearance.value
  if (backstory.value) b.backstory = backstory.value
  return b
})

const preview = useMutation(previewCharacterMutation())
const create = useMutation(createCharacterMutation())
const keep = useMutation(saveCharacterDraftMutation())
const roll = useMutation(rollCharacterScoresMutation())
const discard = useMutation(discardCharacterDraftMutation())
const problem = computed(() => preview.error.value ?? create.error.value ?? roll.error.value)

function restore(d: CharacterDraft) {
  restoring.value = true
  const b = d.build
  name.value = b.name ?? ''
  species.value = b.species ?? ''
  klass.value = b.class ?? ''
  background.value = b.background ?? ''
  if (b.method && methods.value.includes(b.method)) method.value = b.method
  if (b.base) Object.assign(base, b.base)
  const bonuses = Object.entries(b.bonus ?? {}) as [Ability, number][]
  pattern.value = bonuses.some(([, v]) => v === 2) ? 'two-one' : 'one-one-one'
  picks.value = bonuses.sort(([, x], [, y]) => y - x).map(([a]) => a)
  skills.value = b.skills ?? []
  armor.value = b.armor ?? ''
  shield.value = b.shield ?? false
  weapons.value = b.weapons ?? []
  appearance.value = b.appearance ?? ''
  backstory.value = b.backstory ?? ''
  rolled.value = d.rolled ?? []
  step.value = Math.min(d.step, steps.length - 2)
  void Promise.resolve().then(() => (restoring.value = false))
}

onMounted(async () => {
  const found = await getCharacterDraft({ path: { campaignId: id.value } }).catch(() => undefined)
  if (found?.data) restore(found.data)
})

function remember() {
  keep.mutate({ path: { campaignId: id.value }, body: { step: step.value, build: draftBuild.value } })
}
function next() {
  step.value++
  remember()
  if (steps[step.value] === 'Review') preview.mutate({ path: { campaignId: id.value }, body: build.value })
}
function back() {
  step.value--
  remember()
}
function startOver() {
  discard.mutate({ path: { campaignId: id.value } }, { onSuccess: () => {
      restoring.value = true
      name.value = species.value = klass.value = background.value = armor.value = appearance.value = backstory.value = ''
      skills.value = weapons.value = picks.value = []
      shield.value = false
      rolled.value = []
      for (const a of abilities) Reflect.deleteProperty(placed, a)
      Object.assign(base, defaultBase(method.value))
      step.value = 0
      void Promise.resolve().then(() => (restoring.value = false))
    } })
}
function rollScores() {
  roll.mutate({ path: { campaignId: id.value } }, { onSuccess: (d) => (rolled.value = d.rolled ?? []) })
}
function place(a: Ability, index: number) {
  placed[a] = index
  const score = rolled.value[index]
  if (score !== undefined) base[a] = score
}
function save() {
  create.mutate(
    { path: { campaignId: id.value }, body: build.value },
    { onSuccess: (s) => void router.push({ name: 'character', params: { id: id.value, characterId: s.id } }) },
  )
}
function setPick(i: number, a: Ability) {
  const nextPicks = [...picks.value]
  nextPicks[i] = a
  picks.value = nextPicks
}
function adjust(a: Ability, delta: number) {
  base[a] = Math.min(15, Math.max(8, base[a] + delta))
}
</script>

<template>
  <main class="g-page builder">
    <nav aria-label="Breadcrumb" class="g-crumbs">
      <RouterLink :to="{ name: 'campaigns' }">Campaigns</RouterLink> <span aria-hidden="true">›</span>
      <RouterLink :to="{ name: 'campaign', params: { id } }" data-testid="to-campaign">{{ campaign.data.value?.name ?? 'Campaign' }}</RouterLink> <span aria-hidden="true">›</span>
      <span aria-current="page">New character</span>
    </nav>
    <header class="g-headline">
      <span class="g-eyebrow">Character</span>
      <h1>New character</h1>
    </header>
    <p v-if="campaign.isError.value || options.isError.value" role="alert" class="g-alert">The builder could not be loaded. Try again shortly.</p>
    <p v-else-if="!o">Laying out the options…</p>
    <template v-else>
      <p class="hint" data-testid="starting-level">Your Character starts at level {{ startingLevel }} in {{ campaign.data.value?.name }}.</p>
      <ol class="steps" aria-label="Steps">
        <li v-for="(s, i) in steps" :key="s" :aria-current="i === step ? 'step' : undefined" :class="{ done: i < step }">
          <span class="what">{{ s }}<small v-if="chosen[i]" :data-testid="`chosen-${String(i)}`">{{ chosen[i] }}</small></span>
        </li>
      </ol>

      <section v-if="step === 0" class="g-card stack" data-testid="step-species">
        <fieldset>
          <legend>Species</legend>
          <label v-for="s in o.species" :key="s.slug" class="choice">
            <input v-model="species" type="radio" name="species" :value="s.slug" :data-testid="`species-${s.slug}`" />
            <span>{{ s.name }}</span><small>{{ s.speedFeet }} ft</small>
          </label>
        </fieldset>
      </section>

      <section v-else-if="step === 1" class="g-card stack" data-testid="step-class">
        <fieldset>
          <legend>Class</legend>
          <div class="tiles">
            <label v-for="c in o.classes" :key="c.slug" :class="['tile', { picked: klass === c.slug }]">
              <span class="head"><input v-model="klass" type="radio" name="class" :value="c.slug" :data-testid="`class-${c.slug}`" /><strong>{{ c.name }}</strong></span>
              <span class="detail">Hit Die d{{ c.hitDie }}</span>
              <span class="detail">Saves {{ c.saves.map((a) => abbrev[a]).join(', ') }}</span>
              <span class="detail">{{ c.skillChoices }} skills{{ c.caster && c.caster !== 'none' ? ` · ${c.caster === 'pact' ? 'pact magic' : `${c.caster} caster`}` : '' }}</span>
              <span v-if="c.primaryAbilities?.length" class="primary" :data-testid="`primary-${c.slug}`">Main: {{ c.primaryAbilities.map((a) => abbrev[a]).join(' or ') }}</span>
            </label>
          </div>
        </fieldset>
      </section>

      <section v-else-if="step === 2" class="g-card stack" data-testid="step-background">
        <fieldset>
          <legend>Background</legend>
          <label v-for="b in o.backgrounds" :key="b.slug" class="choice">
            <input v-model="background" type="radio" name="background" :value="b.slug" :data-testid="`background-${b.slug}`" />
            <span>{{ b.name }}</span><small>{{ b.skills.map(titleCase).join(', ') }}</small>
          </label>
        </fieldset>
      </section>

      <section v-else-if="step === 3" class="g-card stack" data-testid="step-abilities">
        <label class="g-field">
          <span>Method</span>
          <select v-model="method" data-testid="ability-method">
            <option v-for="m in methods" :key="m" :value="m">{{ methodNames[m] }}</option>
          </select>
        </label>
        <p v-if="method === 'point-buy'" data-testid="points-left">{{ budget - pointsSpent(base) }} of {{ budget }} points left</p>
        <template v-if="method === 'rolled'">
          <GButton v-if="!rolled.length" type="button" :disabled="roll.isPending.value" data-testid="roll-scores" @click="rollScores">Roll six scores</GButton>
          <p v-else data-testid="rolled">Rolled: {{ rolled.join(', ') }}. Place each one once.</p>
        </template>
        <div class="scores">
          <div v-for="a in abilities" :key="a" class="score">
            <span class="abbr">{{ abbrev[a] }}</span>
            <select v-if="method === 'standard-array'" v-model.number="base[a]" :aria-label="titleCase(a)">
              <option v-for="n in standardArray" :key="n" :value="n">{{ n }}</option>
            </select>
            <span v-else-if="method === 'point-buy'" class="stepper">
              <button type="button" :aria-label="`Lower ${a}`" @click="adjust(a, -1)">−</button>
              <span class="value" :data-testid="`score-${a}`">{{ base[a] }}</span>
              <button type="button" :aria-label="`Raise ${a}`" @click="adjust(a, 1)">+</button>
            </span>
            <select
              v-else
              :value="placed[a] ?? ''"
              :aria-label="titleCase(a)"
              :disabled="!rolled.length"
              :data-testid="`place-${a}`"
              @change="place(a, Number(($event.target as HTMLSelectElement).value))"
            >
              <option value="" disabled>–</option>
              <option v-for="(n, i) in rolled" :key="i" :value="i" :disabled="Object.values(placed).includes(i) && placed[a] !== i">{{ n }}</option>
            </select>
            <small>{{ signed(modifier(base[a] + (bonus[a] ?? 0))) }}</small>
          </div>
        </div>
        <fieldset>
          <legend>Background increases</legend>
          <label class="choice"><input v-model="pattern" type="radio" value="two-one" /><span>+2 and +1</span></label>
          <label class="choice"><input v-model="pattern" type="radio" value="one-one-one" /><span>+1 to three</span></label>
          <label v-for="(slot, i) in slots" :key="`${pattern}-${String(i)}`" class="g-field">
            <span>{{ slot }}</span>
            <select :value="picks[i] ?? ''" :aria-label="`Increase ${slot}`" :data-testid="`bonus-${String(i)}`" @change="setPick(i, ($event.target as HTMLSelectElement).value as Ability)">
              <option value="" disabled>Choose</option>
              <option v-for="a in bonusChoices" :key="a" :value="a">{{ titleCase(a) }}</option>
            </select>
          </label>
        </fieldset>
      </section>

      <section v-else-if="step === 4" class="g-card stack" data-testid="step-skills">
        <fieldset>
          <legend>Choose {{ skillLimit }} class skills</legend>
          <p class="hint">From your background: {{ (chosenBackground?.skills ?? []).map(titleCase).join(', ') || 'none' }}</p>
          <label v-for="s in skillOptions" :key="s.skill" class="choice">
            <input
              v-model="skills"
              type="checkbox"
              :value="s.skill"
              :disabled="!skills.includes(s.skill) && skills.length >= skillLimit"
            />
            <span>{{ titleCase(s.skill) }}</span><small>{{ abbrev[s.ability] }}</small>
          </label>
        </fieldset>
      </section>

      <section v-else-if="step === 5" class="g-card stack" data-testid="step-equipment">
        <label class="g-field">
          <span>Armour</span>
          <select v-model="armor" data-testid="armor">
            <option value="">None</option>
            <option v-for="a in o.armor.filter((x) => !x.shield)" :key="a.slug" :value="a.slug">{{ a.name }} (AC {{ a.acBase }})</option>
          </select>
        </label>
        <label class="choice"><input v-model="shield" type="checkbox" data-testid="shield" /><span>Shield</span></label>
        <fieldset>
          <legend>Weapons (up to 4)</legend>
          <label v-for="w in o.weapons" :key="w.slug" class="choice">
            <input v-model="weapons" type="checkbox" :value="w.slug" :disabled="!weapons.includes(w.slug) && weapons.length >= 4" />
            <span>{{ w.name }}</span><small>{{ w.damageDice }} {{ w.damageType }}</small>
          </label>
        </fieldset>
      </section>

      <section v-else-if="step === 6" class="g-card stack" data-testid="step-appearance">
        <GField v-model="appearance" label="What they look like" multiline :maxlength="2000" hint="Height, build, hair, eyes, clothes, scars — whatever the table should picture." data-testid="appearance" />
      </section>

      <section v-else-if="step === 7" class="g-card stack" data-testid="step-story">
        <GField v-model="name" label="Name" :maxlength="60" required data-testid="character-name" />
        <GField v-model="backstory" label="Backstory" multiline :maxlength="4000" data-testid="backstory" />
      </section>

      <section v-else class="g-card stack" data-testid="step-review">
        <p v-if="preview.isPending.value">Checking the rules…</p>
        <template v-else-if="preview.data.value">
          <h2>{{ preview.data.value.name }}</h2>
          <p>Level {{ preview.data.value.level }} {{ preview.data.value.species.name }} {{ preview.data.value.class.name }} · {{ preview.data.value.background.name }}</p>
          <dl class="facts">
            <dt>HP</dt>
            <dd>{{ preview.data.value.hpMax }}</dd>
            <dt>AC</dt>
            <dd>{{ preview.data.value.armorClass }}</dd>
            <dt>Speed</dt>
            <dd>{{ preview.data.value.speedFeet }} ft</dd>
          </dl>
        </template>
      </section>
      <p v-if="problem" role="alert" class="g-alert" data-testid="builder-error">{{ problem.detail ?? 'That build is not allowed.' }}</p>

      <div class="nav">
        <GButton :disabled="step === 0" data-testid="back" @click="back()">Back</GButton>
        <button type="button" class="link" data-testid="start-over" @click="startOver">Start over</button>
        <GButton v-if="step < steps.length - 1" variant="primary" :disabled="!stepValid" data-testid="next" @click="next()">Next</GButton>
        <GButton v-else variant="primary" :disabled="!preview.data.value || create.isPending.value" data-testid="create-character" @click="save()">
          Create character
        </GButton>
      </div>
    </template>
  </main>
</template>

<style scoped>
/* Wide: the steps stand down the side, each with what was chosen, and the step in hand fills the rest. */
@media (min-width: 900px) {
  .builder {
    display: grid;
    grid-template-columns: 250px minmax(0, 1fr);
    column-gap: 40px;
    align-items: start;
  }
  .builder > * {
    grid-column: 2;
  }
  .builder > nav,
  .builder > header,
  .builder > p {
    grid-column: 1 / -1;
  }
  .builder > .steps {
    grid-row: 4 / span 12;
    grid-column: 1;
    flex-direction: column;
    flex-wrap: nowrap;
    gap: 2px;
  }
}
.steps {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
  counter-reset: step;
  color: var(--color-text-2);
}
.steps li {
  display: grid;
  grid-template-columns: 26px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border-radius: var(--radius-control);
  counter-increment: step;
}
.steps li::before {
  display: grid;
  place-items: center;
  box-sizing: border-box;
  width: 24px;
  height: 24px;
  border: 1px solid var(--color-edge);
  border-radius: 50%;
  font-size: 12px;
  content: counter(step);
}
.steps li.done::before {
  border-color: var(--color-gold);
  color: var(--color-gold-high);
  content: '◆';
}
.steps li[aria-current='step'] {
  color: var(--color-gold-high);
  background: var(--color-selected);
}
.steps li[aria-current='step']::before {
  border-color: var(--color-brass-edge);
}
.what {
  display: flex;
  flex-direction: column;
  font-family: var(--font-label);
  font-size: 15px;
}
.what small {
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--color-text-3);
}
section.g-card {
  margin: 0;
  padding: 0;
  border: 0;
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
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
  margin-bottom: 10px;
  padding: 0;
  font-family: var(--font-label);
  font-size: 15px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
/* A choice is a row under a rule; the one taken is lit. */
.choice {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 46px;
  padding: 0 10px;
  border-top: 1px solid var(--color-rule);
  cursor: pointer;
}
fieldset > .choice + :not(.choice) {
  margin-top: 6px;
}
.choice:has(input:checked) {
  color: var(--color-gold-high);
  background: var(--color-selected);
}
.choice small {
  margin-left: auto;
  font-size: 13px;
  color: var(--color-text-3);
}
.scores {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(96px, 1fr));
  gap: 8px;
}
.score {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 10px 8px;
  border-radius: var(--radius-control);
  background: var(--color-field);
}
.score select,
.score input {
  min-height: 44px;
  width: 64px;
  font-size: 16px;
  text-align: center;
  border: 0;
  border-radius: var(--radius-control) var(--radius-control) 0 0;
  color: var(--color-text);
  background: var(--color-inset);
  box-shadow: inset 0 -1px 0 var(--color-line);
}
.abbr {
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.stepper {
  display: flex;
  align-items: center;
  gap: 6px;
}
.stepper button {
  min-width: 44px;
  min-height: 44px;
  border: 1px solid var(--color-edge);
  border-radius: var(--radius-control);
  background: var(--color-raised);
  color: var(--color-text);
  font-size: 18px;
  cursor: pointer;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 12px;
}
dd {
  margin: 0;
}
.nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.link {
  border: 0;
  background: none;
  color: var(--color-text-3);
  cursor: pointer;
  text-decoration: underline;
}
.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 190px), 1fr));
  gap: 8px;
}
.tile {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 12px 12px;
  border: 1px solid transparent;
  border-radius: var(--radius-control);
  background: var(--color-field);
  cursor: pointer;
}
.tile strong {
  font-family: var(--font-display);
  font-size: 15px;
}
.tile.picked {
  border-color: var(--color-brass-edge);
  background: var(--color-selected);
}
.tile:focus-within {
  outline: 2px solid var(--color-gold);
}
.detail {
  font-size: 13px;
  color: var(--color-text-2);
}
.primary {
  color: var(--color-gold-high);
  font-size: 14px;
}
.tile .head {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
