<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createCharacterMutation,
  getBuilderOptionsOptions,
  getCampaignOptions,
  previewCharacterMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Ability, AbilityBase, CharacterBuild } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
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
    ...getBuilderOptionsOptions({ query: { ruleset: ruleset.value ?? 'srd-2024' } }),
    enabled: ruleset.value !== undefined,
  })),
)
const o = computed(() => options.data.value)

const steps = ['Origin', 'Class', 'Abilities', 'Skills', 'Equipment', 'Review'] as const
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

watch(method, (m) => Object.assign(base, defaultBase(m)))
watch(background, () => (picks.value = []))
watch(klass, () => (skills.value = []))

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

const stepValid = computed(() => {
  switch (step.value) {
    case 0:
      return name.value.trim() !== '' && species.value !== '' && background.value !== ''
    case 1:
      return klass.value !== ''
    case 2:
      return baseValid(method.value, base, budget.value) && bonusValid(bonus.value)
    case 3:
      return skills.value.length === skillLimit.value
    default:
      return weapons.value.length <= 4
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
}))

const preview = useMutation(previewCharacterMutation())
const create = useMutation(createCharacterMutation())
const problem = computed(() => preview.error.value ?? create.error.value)

function next() {
  step.value++
  if (steps[step.value] === 'Review') preview.mutate({ path: { campaignId: id.value }, body: build.value })
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
    <RouterLink :to="{ name: 'campaign', params: { id } }" class="back">← Campaign</RouterLink>
    <h1>New character</h1>
    <p v-if="campaign.isError.value || options.isError.value" role="alert" class="g-alert">The builder could not be loaded. Try again shortly.</p>
    <p v-else-if="!o">Laying out the options…</p>
    <template v-else>
      <ol class="steps" aria-label="Steps">
        <li v-for="(s, i) in steps" :key="s" :aria-current="i === step ? 'step' : undefined" :class="{ done: i < step }">{{ s }}</li>
      </ol>

      <section v-if="step === 0" class="g-card stack" data-testid="step-origin">
        <label class="g-field">
          <span>Name</span>
          <input v-model="name" maxlength="60" data-testid="character-name" />
        </label>
        <fieldset>
          <legend>Species</legend>
          <label v-for="s in o.species" :key="s.slug" class="choice">
            <input v-model="species" type="radio" name="species" :value="s.slug" />
            <span>{{ s.name }}</span><small>{{ s.speedFeet }} ft</small>
          </label>
        </fieldset>
        <fieldset>
          <legend>Background</legend>
          <label v-for="b in o.backgrounds" :key="b.slug" class="choice">
            <input v-model="background" type="radio" name="background" :value="b.slug" />
            <span>{{ b.name }}</span><small>{{ b.skills.map(titleCase).join(', ') }}</small>
          </label>
        </fieldset>
      </section>

      <section v-else-if="step === 1" class="g-card stack" data-testid="step-class">
        <fieldset>
          <legend>Class</legend>
          <label v-for="c in o.classes" :key="c.slug" class="choice">
            <input v-model="klass" type="radio" name="class" :value="c.slug" />
            <span>{{ c.name }}</span><small>d{{ c.hitDie }} · saves {{ c.saves.map((a) => abbrev[a]).join(', ') }}</small>
          </label>
        </fieldset>
      </section>

      <section v-else-if="step === 2" class="g-card stack" data-testid="step-abilities">
        <label class="g-field">
          <span>Method</span>
          <select v-model="method" data-testid="ability-method">
            <option value="standard-array">Standard array</option>
            <option value="point-buy">Point buy</option>
            <option value="rolled">Rolled (4d6, drop lowest)</option>
          </select>
        </label>
        <p v-if="method === 'point-buy'" data-testid="points-left">{{ budget - pointsSpent(base) }} of {{ budget }} points left</p>
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
            <input v-else v-model.number="base[a]" type="number" min="3" max="18" :aria-label="titleCase(a)" />
            <small>{{ signed(modifier(base[a] + (bonus[a] ?? 0))) }}</small>
          </div>
        </div>
        <fieldset>
          <legend>Origin increases</legend>
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

      <section v-else-if="step === 3" class="g-card stack" data-testid="step-skills">
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

      <section v-else-if="step === 4" class="g-card stack" data-testid="step-equipment">
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

      <section v-else class="g-card stack" data-testid="step-review">
        <p v-if="preview.isPending.value">Checking the rules…</p>
        <template v-else-if="preview.data.value">
          <h2>{{ preview.data.value.name }}</h2>
          <p>{{ preview.data.value.species.name }} {{ preview.data.value.class.name }} · {{ preview.data.value.background.name }}</p>
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
        <GButton :disabled="step === 0" @click="step--">Back</GButton>
        <GButton v-if="step < steps.length - 1" variant="primary" :disabled="!stepValid" data-testid="next" @click="next()">Next</GButton>
        <GButton v-else variant="primary" :disabled="!preview.data.value || create.isPending.value" data-testid="create-character" @click="save()">
          Create character
        </GButton>
      </div>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.steps {
  display: flex;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-x: auto;
  font-size: 13px;
  color: var(--color-text-3);
}
.steps li {
  flex: none;
  padding: 4px 10px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-line);
}
.steps li[aria-current='step'] {
  border-color: var(--color-gold);
  color: var(--color-gold-high);
}
.steps li.done {
  color: var(--color-text-2);
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
  padding: 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
}
.score select,
.score input {
  min-height: 44px;
  width: 64px;
  font-size: 16px;
  text-align: center;
  background: var(--color-surface);
  color: var(--color-text);
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
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
  border: 1px solid var(--color-bronze);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font-size: 18px;
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
  justify-content: space-between;
  gap: 8px;
}
</style>
