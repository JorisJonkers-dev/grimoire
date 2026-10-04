<script setup lang="ts">
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { levelUpMutation, planLevelUpOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Ability, AbilityBonus, LevelUpChoice } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { abbrev, abilities } from './build'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const ids = computed(() => ({ campaignId: String(route.params.id), characterId: String(route.params.characterId) }))
const klass = ref('')
const picks = reactive<Record<string, string[]>>({})
// A subclass picked on this level asks for its own choices, so the plan is read again with it.
const subclass = computed(() => picks.subclass?.[0] ?? '')
const plan = useQuery(
  computed(() => ({
    ...planLevelUpOptions({ path: ids.value, query: { ...(klass.value ? { class: klass.value } : {}), ...(subclass.value ? { subclass: subclass.value } : {}) } }),
    retry: false,
    placeholderData: keepPreviousData,
  })),
)
const p = computed(() => plan.data.value)
const take = useMutation(levelUpMutation())

const spells = ref<string[]>([])
const hitPoints = ref<'average' | 'roll'>('average')
const asiFirst = ref<Ability | ''>('')
const asiSecond = ref<Ability | ''>('')
const step = ref(0)

watch(
  () => p.value?.class,
  (c) => {
    if (c && !klass.value) klass.value = c
    for (const k of Object.keys(picks)) Reflect.deleteProperty(picks, k)
    spells.value = []
  },
)

const steps = computed(() => {
  const out = ['Class']
  if (p.value?.choices.length) out.push('Choices')
  if (p.value && p.value.cantrips + p.value.spells > 0) out.push('Spells')
  out.push('Hit points', 'Review')
  return out
})
const current = computed(() => steps.value[step.value] ?? 'Class')
const cantripList = computed(() => p.value?.spellList.filter((s) => s.level === 0) ?? [])
const spellList = computed(() => p.value?.spellList.filter((s) => s.level > 0) ?? [])
const chosenCantrips = computed(() => spells.value.filter((s) => cantripList.value.some((c) => c.slug === s)).length)
const chosenSpells = computed(() => spells.value.length - chosenCantrips.value)
const takesAsi = computed(() => Object.values(picks).some((v) => v.includes('ability-score-improvement')))

const increase = computed<AbilityBonus>(() => {
  if (!takesAsi.value || !asiFirst.value) return {}
  if (!asiSecond.value || asiSecond.value === asiFirst.value) return { [asiFirst.value]: 2 }
  return { [asiFirst.value]: 1, [asiSecond.value]: 1 }
})

const choicesDone = computed(() => (p.value?.choices ?? []).every((c) => (picks[c.slug]?.length ?? 0) === c.count))
const stepValid = computed(() => {
  switch (current.value) {
    case 'Choices':
      return choicesDone.value && (!takesAsi.value || asiFirst.value !== '')
    case 'Spells':
      return chosenCantrips.value === p.value?.cantrips && chosenSpells.value === p.value.spells
    default:
      return true
  }
})

function toggle(choice: LevelUpChoice, slug: string) {
  const now = picks[choice.slug] ?? []
  if (choice.count === 1) picks[choice.slug] = [slug]
  else if (now.includes(slug)) picks[choice.slug] = now.filter((v) => v !== slug)
  else if (now.length < choice.count) picks[choice.slug] = [...now, slug]
}
function toggleSpell(slug: string, cantrip: boolean) {
  if (spells.value.includes(slug)) spells.value = spells.value.filter((s) => s !== slug)
  else if (cantrip ? chosenCantrips.value < (p.value?.cantrips ?? 0) : chosenSpells.value < (p.value?.spells ?? 0)) spells.value = [...spells.value, slug]
}
function chooseClass(slug: string) {
  klass.value = slug
  step.value = 0
}
function submit() {
  const body = {
    class: klass.value,
    hitPoints: hitPoints.value,
    picks: Object.entries(picks)
      .filter(([choice]) => p.value?.choices.some((c) => c.slug === choice))
      .map(([choice, values]) => ({ choice, values })),
    increase: increase.value,
    spells: spells.value,
  }
  take.mutate(
    { path: ids.value, body },
    {
      onSuccess: () => {
        void client.invalidateQueries()
        void router.push({ name: 'character', params: { id: ids.value.campaignId, characterId: ids.value.characterId } })
      },
    },
  )
}
const nameOf = (slug: string) => p.value?.spellList.find((s) => s.slug === slug)?.name ?? slug
const optionName = (choice: LevelUpChoice, slug: string) => choice.options.find((o) => o.slug === slug)?.name ?? slug
</script>

<template>
  <main class="g-page level-up">
    <RouterLink :to="{ name: 'character', params: { id: ids.campaignId, characterId: ids.characterId } }" class="back">← Sheet</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Character</span>
      <h1>Level up</h1>
    </header>
    <p v-if="plan.isError.value" role="alert" class="g-alert" data-testid="level-up-error">
      {{ plan.error.value?.detail ?? 'The next level could not be loaded.' }}
    </p>
    <p v-else-if="!p">Turning the page…</p>
    <template v-else>
      <p v-if="!p.ready" class="g-alert" data-testid="level-up-locked">
        {{ p.held ? 'Your DM holds level-ups: the next level opens when they grant it.' : 'The next level opens after a long rest, or when your DM grants it.' }}
      </p>
      <p class="hint">Level {{ p.level }}: level {{ p.classLevel }} as {{ p.classes.find((c) => c.slug === p?.class)?.name }}.</p>
      <ol class="steps" aria-label="Steps">
        <li v-for="(s, i) in steps" :key="s" :aria-current="i === step ? 'step' : undefined" :class="{ done: i < step }">{{ s }}</li>
      </ol>

      <section v-if="current === 'Class'" class="g-card stack" data-testid="step-class">
        <fieldset>
          <legend>Class</legend>
          <div class="tiles">
            <label v-for="c in p.classes" :key="c.slug" :class="['tile', { picked: p.class === c.slug, blocked: c.unmet.length > 0 }]">
              <span class="head">
                <input
                  type="radio"
                  name="class"
                  :value="c.slug"
                  :checked="p.class === c.slug"
                  :disabled="c.unmet.length > 0"
                  :data-testid="`level-class-${c.slug}`"
                  @change="chooseClass(c.slug)"
                />
                <strong>{{ c.name }}</strong>
              </span>
              <span class="detail">{{ c.level ? `Level ${String(c.level)} now` : 'New class' }} · d{{ c.hitDie }}</span>
              <span v-if="c.unmet.length" class="detail unmet">Needs {{ c.unmet.join(', ') }}</span>
            </label>
          </div>
        </fieldset>
      </section>

      <section v-else-if="current === 'Choices'" class="g-card stack" data-testid="step-choices">
        <fieldset v-for="c in p.choices" :key="c.slug" :data-testid="`choice-${c.slug}`">
          <legend>{{ c.name }}<small v-if="c.count > 1"> · choose {{ c.count }}</small></legend>
          <label v-for="o in c.options" :key="o.slug" class="choice" :class="{ blocked: o.unmet.length > 0 }">
            <input
              :type="c.count === 1 ? 'radio' : 'checkbox'"
              :name="c.slug"
              :value="o.slug"
              :checked="picks[c.slug]?.includes(o.slug) ?? false"
              :disabled="o.unmet.length > 0"
              :data-testid="`pick-${c.slug}-${o.slug}`"
              @change="toggle(c, o.slug)"
            />
            <span>{{ o.name }}</span>
            <small v-if="o.unmet.length">Needs {{ o.unmet.join(', ') }}</small>
          </label>
        </fieldset>
        <fieldset v-if="takesAsi" data-testid="asi">
          <legend>Ability Score Improvement</legend>
          <p class="hint">Raise one ability by 2, or two abilities by 1 each. None goes past 20.</p>
          <label class="pick">
            <span>First ability</span>
            <select v-model="asiFirst" data-testid="asi-first">
              <option value="">Choose</option>
              <option v-for="a in abilities" :key="a" :value="a">{{ abbrev[a] }}</option>
            </select>
          </label>
          <label class="pick">
            <span>Second ability (leave empty for +2 to the first)</span>
            <select v-model="asiSecond" data-testid="asi-second">
              <option value="">None</option>
              <option v-for="a in abilities" :key="a" :value="a">{{ abbrev[a] }}</option>
            </select>
          </label>
        </fieldset>
      </section>

      <section v-else-if="current === 'Spells'" class="g-card stack" data-testid="step-spells">
        <fieldset v-if="p.cantrips > 0">
          <legend>Cantrips · {{ chosenCantrips }} of {{ p.cantrips }}</legend>
          <label v-for="s in cantripList" :key="s.slug" class="choice">
            <input type="checkbox" :checked="spells.includes(s.slug)" :data-testid="`spell-${s.slug}`" @change="toggleSpell(s.slug, true)" />
            <span>{{ s.name }}</span>
          </label>
        </fieldset>
        <fieldset v-if="p.spells > 0">
          <legend>Spells · {{ chosenSpells }} of {{ p.spells }}</legend>
          <label v-for="s in spellList" :key="s.slug" class="choice">
            <input type="checkbox" :checked="spells.includes(s.slug)" :data-testid="`spell-${s.slug}`" @change="toggleSpell(s.slug, false)" />
            <span>{{ s.name }}</span><small>Level {{ s.level }}</small>
          </label>
        </fieldset>
      </section>

      <section v-else-if="current === 'Hit points'" class="g-card stack" data-testid="step-hp">
        <fieldset>
          <legend>Hit points</legend>
          <label class="choice">
            <input v-model="hitPoints" type="radio" name="hp" value="average" data-testid="hp-average" />
            <span>Take the average</span><small>+{{ p.average }}</small>
          </label>
          <label class="choice">
            <input v-model="hitPoints" type="radio" name="hp" value="roll" data-testid="hp-roll" />
            <span>Roll the d{{ p.hitDie }}</span><small>at least 1</small>
          </label>
        </fieldset>
      </section>

      <section v-else class="g-card stack" data-testid="step-review">
        <h2>Level {{ p.level }}</h2>
        <dl class="facts">
          <dt>Class</dt>
          <dd>{{ p.classes.find((c) => c.slug === p?.class)?.name }} {{ p.classLevel }}</dd>
          <template v-for="c in p.choices" :key="c.slug">
            <dt>{{ c.name }}</dt>
            <dd>{{ (picks[c.slug] ?? []).map((v) => optionName(c, v)).join(', ') }}</dd>
          </template>
          <template v-if="takesAsi">
            <dt>Abilities</dt>
            <dd>{{ Object.entries(increase).map(([a, n]) => `${abbrev[a as Ability]} +${String(n)}`).join(', ') }}</dd>
          </template>
          <template v-if="spells.length">
            <dt>Learns</dt>
            <dd>{{ spells.map(nameOf).join(', ') }}</dd>
          </template>
          <dt>Hit points</dt>
          <dd>{{ hitPoints === 'average' ? `+${String(p.average)}` : `d${String(p.hitDie)} roll` }}</dd>
        </dl>
      </section>
      <p v-if="take.error.value" role="alert" class="g-alert" data-testid="level-up-problem">
        {{ take.error.value.detail ?? 'That level was not taken.' }}
      </p>

      <div class="nav">
        <GButton :disabled="step === 0" data-testid="back" @click="step--">Back</GButton>
        <GButton v-if="step < steps.length - 1" variant="primary" :disabled="!stepValid" data-testid="next" @click="step++">Next</GButton>
        <GButton v-else variant="primary" :disabled="!p.ready || take.isPending.value" data-testid="take-level" @click="submit()">
          Take level {{ p.level }}
        </GButton>
      </div>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.level-up {
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
.steps {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 13px;
  color: var(--color-text-3);
}
.steps li {
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
.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 180px), 1fr));
  gap: 8px;
}
.tile {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  cursor: pointer;
}
.tile.picked {
  border-color: var(--color-gold);
  background: var(--color-raised);
}
.tile:focus-within {
  outline: 2px solid var(--color-gold);
}
.tile .head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.detail {
  font-size: 13px;
  color: var(--color-text-2);
}
.blocked {
  cursor: not-allowed;
}
.unmet,
.choice.blocked small {
  color: var(--color-enemy-soft);
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
.pick {
  display: flex;
  flex-direction: column;
  gap: 2px;
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
.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 12px;
  margin: 0;
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
</style>
