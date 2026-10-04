<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { createRecipeMutation, deleteRecipeMutation, getDowntimeOptions, grantDowntimeMutation, spendDowntimeMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { DowntimeCharacter, DowntimeEntry, DowntimeKind, Problem, Recipe } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

// Downtime: the days each Character has between adventures, and what they do with them. The DM gives
// the days and keeps the Recipes; a Player spends their own Character's days.
const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const downtime = useQuery({ ...getDowntimeOptions(path), retry: false })
const d = computed(() => downtime.data.value)
const dm = computed(() => d.value?.dm ?? false)

const plural = (n: number, one: string) => `${String(n)} ${one}${n === 1 ? '' : 's'}`
const two = (n: number) => String(n).padStart(2, '0')
const clock = computed(() => `The Game Clock stands at day ${String(d.value?.gameDay ?? 0)}, ${two(Math.floor((d.value?.gameMinute ?? 0) / 60))}:${two((d.value?.gameMinute ?? 0) % 60)}.`)
// A price in copper, as gold, silver and copper.
function price(cp: number): string {
  const parts: string[] = []
  for (const [name, value] of [['gp', 100], ['sp', 10], ['cp', 1]] as const) {
    const n = Math.floor(cp / value)
    if (n > 0) parts.push(`${String(n)} ${name}`)
    cp -= n * value
  }
  return parts.join(' ')
}
const listed = (parts: string[]) => (parts.length > 1 ? `${parts.slice(0, -1).join(', ')} and ${parts.at(-1) ?? ''}` : parts.join(''))
function says(r: Recipe): string {
  const cost = r.costCp > 0 ? ` for ${price(r.costCp)}` : ''
  const from = r.ingredients.length > 0 ? `, from ${listed(r.ingredients.map((i) => `${String(i.count)} × ${i.item}`))}` : ''
  const tool = r.tool ? `, with a ${r.tool} at hand` : ''
  return `${r.name}: makes ${String(r.quantity)} × ${r.makes} in ${plural(r.days, 'day')}${cost}${from}${tool}.`
}
function did(e: DowntimeEntry): string {
  const days = plural(e.days, 'day')
  if (e.activity === 'craft') return `${e.character} crafted ${e.detail} over ${days}.`
  if (e.activity === 'work') return `${e.character} worked for ${days}.`
  return `${e.character} ${e.activity === 'train' ? 'trained in' : 'researched'} ${e.detail} for ${days}.`
}

const problem = ref('')
const news = ref('')
const failed = (e: Problem) => {
  news.value = ''
  problem.value = e.detail ?? 'That could not be done.'
}
const done = (what: string) => {
  problem.value = ''
  news.value = what
  void client.invalidateQueries()
}

// How each Character is about to spend its days.
type Plan = { activity: DowntimeKind; recipe: string; days: number; subject: string }
const plans = reactive<Record<string, Plan>>({})
const planOf = (ch: DowntimeCharacter) => (plans[ch.id] ??= { activity: 'craft', recipe: d.value?.recipes[0]?.id ?? '', days: 1, subject: '' })
const ready = (p: Plan) => (p.activity === 'craft' ? p.recipe !== '' : p.activity === 'work' || p.subject.trim() !== '')
const spending = useMutation(spendDowntimeMutation())
function spend(ch: DowntimeCharacter) {
  const p = planOf(ch)
  const body = p.activity === 'craft'
    ? { activity: p.activity, recipeId: p.recipe }
    : { activity: p.activity, days: p.days, ...(p.activity === 'work' ? {} : { subject: p.subject.trim() }) }
  spending.mutate({ path: { campaignId, characterId: ch.id }, body }, { onSuccess: () => { done(`${ch.name} spent the time.`) }, onError: failed })
}

const grant = reactive({ days: 1, who: '' })
const granting = useMutation(grantDowntimeMutation())
function give() {
  const name = d.value?.characters.find((ch) => ch.id === grant.who)?.name ?? 'Everyone'
  const days = grant.days
  granting.mutate({ ...path, body: { days, ...(grant.who ? { characterId: grant.who } : {}) } }, {
    onSuccess: () => { done(`${name} has ${String(days)} more downtime ${days === 1 ? 'day' : 'days'}.`) },
    onError: failed,
  })
}

const fresh = () => ({ name: '', makes: '', quantity: 1, days: 1, cost: 0, tool: '', ingredients: '' })
const draft = reactive(fresh())
// Each line names an ingredient, with how many of it in front: "2 healing-herb".
function ingredients(text: string) {
  return text.split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
    const m = /^(\d+)\s+(.+)$/.exec(line)
    return m ? { item: (m[2] ?? '').trim(), count: Number(m[1]) } : { item: line, count: 1 }
  })
}
const adding = useMutation(createRecipeMutation())
function add() {
  const tool = draft.tool.trim()
  const body = {
    name: draft.name.trim(), makes: draft.makes.trim(), quantity: draft.quantity, days: draft.days, costCp: Math.round(draft.cost * 100),
    ...(tool ? { tool } : {}), ingredients: ingredients(draft.ingredients),
  }
  adding.mutate({ ...path, body }, {
    onSuccess: () => {
      Object.assign(draft, fresh())
      done('The Recipe is added.')
    },
    onError: failed,
  })
}
const removing = useMutation(deleteRecipeMutation())
const remove = (r: Recipe) => { removing.mutate({ path: { campaignId, recipeId: r.id } }, { onSuccess: () => { done(`${r.name} is removed.`) }, onError: failed }) }
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Campaign</span>
      <h1>Downtime</h1>
    </header>
    <p v-if="downtime.isError.value" role="alert" class="g-alert" data-testid="downtime-missing">That Campaign is not available.</p>
    <template v-else-if="d">
      <p class="hint" data-testid="downtime-clock">{{ clock }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="downtime-problem">{{ problem }}</p>
      <p v-if="news" role="status" class="news" data-testid="downtime-done">{{ news }}</p>

      <section class="g-card part" aria-labelledby="days-title">
        <h2 id="days-title">Downtime days</h2>
        <p class="hint">Days are spent between Sessions. The Game Clock moves on as they are, once for days Characters spend side by side.</p>
        <div v-for="ch in d.characters" :key="ch.id" class="part">
          <p class="who" data-testid="downtime-days">{{ ch.name }}: {{ plural(ch.days, 'downtime day') }}</p>
          <form v-if="ch.mine" class="row" :aria-label="`Spend ${ch.name}'s downtime`" :data-testid="`spend-${ch.id}`" @submit.prevent="spend(ch)">
            <label class="g-field">
              <span>Spend it</span>
              <select v-model="planOf(ch).activity" data-testid="activity">
                <option value="craft">Crafting</option>
                <option value="work">Working</option>
                <option value="train">Training</option>
                <option value="research">Researching</option>
              </select>
            </label>
            <label v-if="planOf(ch).activity === 'craft'" class="g-field">
              <span>Recipe</span>
              <select v-model="planOf(ch).recipe" data-testid="recipe">
                <option v-for="r in d.recipes" :key="r.id" :value="r.id">{{ r.name }}</option>
              </select>
            </label>
            <template v-else>
              <label class="g-field small"><span>Days</span><input v-model.number="planOf(ch).days" type="number" min="1" max="3650" data-testid="days" /></label>
              <label v-if="planOf(ch).activity !== 'work'" class="g-field grow"><span>In what</span><input v-model="planOf(ch).subject" maxlength="80" data-testid="subject" /></label>
            </template>
            <GButton type="submit" variant="primary" :disabled="!ready(planOf(ch))">Spend</GButton>
          </form>
        </div>
        <form v-if="dm" class="row" aria-label="Give downtime days" data-testid="downtime-grant" @submit.prevent="give()">
          <label class="g-field small"><span>Give days</span><input v-model.number="grant.days" type="number" min="1" max="3650" data-testid="grant-days" /></label>
          <label class="g-field">
            <span>To</span>
            <select v-model="grant.who" data-testid="grant-who">
              <option value="">Everyone: a new downtime</option>
              <option v-for="ch in d.characters" :key="ch.id" :value="ch.id">{{ ch.name }}</option>
            </select>
          </label>
          <GButton type="submit">Give</GButton>
        </form>
      </section>

      <section class="g-card part" aria-labelledby="recipes-title">
        <h2 id="recipes-title">Recipes</h2>
        <p v-if="d.recipes.length === 0" class="hint" data-testid="no-recipes">No Recipes in this Campaign yet.</p>
        <ul v-else class="g-list">
          <li v-for="r in d.recipes" :key="r.id" class="row" :data-testid="`recipe-${r.id}`">
            <span data-testid="recipe-says">{{ says(r) }}</span>
            <GButton v-if="dm" variant="danger" :aria-label="`Remove ${r.name}`" data-testid="recipe-remove" @click="remove(r)">Remove</GButton>
          </li>
        </ul>
        <form v-if="dm" class="part" aria-label="Add a Recipe" data-testid="recipe-add" @submit.prevent="add()">
          <h3>Add a Recipe</h3>
          <label class="g-field"><span>Name</span><input v-model="draft.name" maxlength="80" data-testid="recipe-name" /></label>
          <div class="row">
            <label class="g-field grow"><span>Item it makes, by slug</span><input v-model="draft.makes" maxlength="80" data-testid="recipe-makes" /></label>
            <label class="g-field small"><span>How many</span><input v-model.number="draft.quantity" type="number" min="1" max="100" data-testid="recipe-quantity" /></label>
            <label class="g-field small"><span>Days</span><input v-model.number="draft.days" type="number" min="1" max="365" data-testid="recipe-days" /></label>
            <label class="g-field small"><span>Cost in gp</span><input v-model.number="draft.cost" type="number" min="0" step="0.01" data-testid="recipe-cost" /></label>
          </div>
          <label class="g-field"><span>Tool it needs at hand, by slug</span><input v-model="draft.tool" maxlength="80" data-testid="recipe-tool" /></label>
          <label class="g-field"><span>Ingredients, one on each line, as "2 healing-herb"</span><textarea v-model="draft.ingredients" rows="3" data-testid="recipe-ingredients"></textarea></label>
          <GButton type="submit" variant="primary" :disabled="draft.name.trim() === '' || draft.makes.trim() === ''">Add Recipe</GButton>
        </form>
      </section>

      <section v-if="d.log.length > 0" class="g-card part" aria-labelledby="log-title">
        <h2 id="log-title">What was done</h2>
        <ul class="g-list" data-testid="downtime-log">
          <li v-for="e in d.log" :key="e.id">{{ did(e) }}</li>
        </ul>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.hint,
.who,
.news {
  margin: 0;
}
.hint {
  color: var(--color-text-2);
}
.who {
  font-weight: 700;
}
.news {
  color: var(--color-gold-high);
}
.part {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.part h2,
.part h3 {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.small {
  max-width: 120px;
}
.grow {
  flex: 1;
  min-width: 160px;
}
</style>
