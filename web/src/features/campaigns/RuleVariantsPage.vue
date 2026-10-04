<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  createRuleHookMutation,
  deleteRuleHookMutation,
  getCampaignOptions,
  listRuleHooksOptions,
  listRuleVariantsOptions,
  setRuleVariantsMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { HookPointSlug, RuleHook, RuleVariant } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

// The Campaign's Rule Variants: every Member sees how the table plays, and the DM switches each one.
const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const campaign = useQuery({ ...getCampaignOptions(path), retry: false })
const dm = computed(() => campaign.data.value?.myRole === 'dm')
const list = useQuery({ ...listRuleVariantsOptions(path), retry: false })
const variants = computed(() => list.data.value ?? [])

const labelOf = (v: RuleVariant) => v.options.find((o) => o.value === v.value)?.label ?? v.value
// A variant is on when it is at anything but how the rules play without it.
const isOn = (v: RuleVariant) => v.value !== v.options[0]?.value
const problem = ref('')
const saved = ref('')
const change = useMutation(setRuleVariantsMutation())
function pick(v: RuleVariant, value: string) {
  change.mutate({ ...path, body: { choices: [{ slug: v.slug, value }] } }, {
    onSuccess: () => {
      problem.value = ''
      saved.value = `${v.name} is switched.`
      void client.invalidateQueries()
    },
    onError: () => {
      saved.value = ''
      problem.value = 'That could not be switched. Try again.'
    },
  })
}

// The Campaign's own Rule Variants: at a hook point each rolls on a Roll Table or applies an Effect.
const own = useQuery({ ...listRuleHooksOptions(path), retry: false })
const hooks = computed(() => own.data.value?.hooks ?? [])
const points = computed(() => own.data.value?.points ?? [])
const tables = computed(() => own.data.value?.tables ?? [])
const pointLabel = (slug: string) => points.value.find((p) => p.slug === slug)?.label ?? slug
function says(h: RuleHook): string {
  if (!h.rollTableId) return `${h.name}: ${pointLabel(h.hook)}, ${h.effect ?? ''} applies.`
  return `${h.name}: ${pointLabel(h.hook)}, roll on ${h.tableName || 'a Roll Table this Campaign no longer sees'}.`
}
const failed = () => {
  saved.value = ''
  problem.value = 'That could not be done. Check what you entered and try again.'
}
const did = (what: string) => {
  problem.value = ''
  saved.value = what
  void client.invalidateQueries()
}
const draft = reactive<{ name: string; hook: HookPointSlug; table: string; effect: string }>({ name: '', hook: 'natural-1', table: '', effect: '' })
const ready = computed(() => draft.name.trim() !== '' && (draft.table !== '' || draft.effect.trim() !== ''))
const create = useMutation(createRuleHookMutation())
function add() {
  const name = draft.name.trim()
  const outcome = draft.table ? { rollTableId: draft.table } : { effect: draft.effect.trim() }
  create.mutate({ ...path, body: { name, hook: draft.hook, ...outcome } }, {
    onSuccess: () => {
      Object.assign(draft, { name: '', hook: 'natural-1', table: '', effect: '' })
      did(`${name} is added.`)
    },
    onError: failed,
  })
}
const drop = useMutation(deleteRuleHookMutation())
const remove = (h: RuleHook) => { drop.mutate({ path: { campaignId, hookId: h.id } }, { onSuccess: () => { did(`${h.name} is removed.`) }, onError: failed }) }
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Campaign</span>
      <h1>Rule Variants</h1>
    </header>
    <p v-if="list.isError.value" role="alert" class="g-alert" data-testid="variants-missing">That Campaign is not available.</p>
    <template v-else>
      <p class="hint">Optional rules this table plays with. A Session under way follows a switch at once.</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="variants-problem">{{ problem }}</p>
      <p v-if="saved" role="status" class="saved" data-testid="variants-saved">{{ saved }}</p>
      <section v-for="v in variants" :key="v.slug" class="g-card variant" :aria-labelledby="`variant-title-${v.slug}`" :data-testid="`variant-${v.slug}`" :data-on="isOn(v)">
        <h2 :id="`variant-title-${v.slug}`">{{ v.name }}</h2>
        <p class="words" data-testid="variant-description">{{ v.description }}</p>
        <label v-if="dm" class="g-field">
          <span>{{ v.name }} is</span>
          <select :value="v.value" @change="pick(v, ($event.target as HTMLSelectElement).value)">
            <option v-for="o in v.options" :key="o.value" :value="o.value">{{ o.label }}</option>
          </select>
        </label>
        <p v-else class="value" data-testid="variant-value">{{ labelOf(v) }}</p>
        <p class="hint" data-testid="variant-applied">{{ v.automated ? 'Grimoire applies this in play.' : 'The DM applies this by hand.' }}</p>
      </section>

      <section v-if="own.isSuccess.value" class="g-card variant" aria-labelledby="own-title">
        <h2 id="own-title">This Campaign's own</h2>
        <p class="hint">Rule Variants the DM authored: when something happens in play, whoever it happened to rolls on a Roll Table or takes an Effect.</p>
        <p v-if="hooks.length === 0" class="hint" data-testid="no-hooks">This Campaign has no Rule Variants of its own yet.</p>
        <ul v-else class="g-list">
          <li v-for="h in hooks" :key="h.id" class="hook" :data-testid="`hook-${h.id}`">
            <span>{{ says(h) }}</span>
            <GButton v-if="dm" variant="danger" :aria-label="`Remove ${h.name}`" data-testid="hook-remove" @click="remove(h)">Remove</GButton>
          </li>
        </ul>
        <form v-if="dm" class="own" aria-label="Author a Rule Variant" data-testid="hook-add" @submit.prevent="add()">
          <label class="g-field"><span>Name</span><input v-model="draft.name" maxlength="80" data-testid="hook-name" /></label>
          <label class="g-field">
            <span>When</span>
            <select v-model="draft.hook" data-testid="hook-point">
              <option v-for="p in points" :key="p.slug" :value="p.slug">{{ p.label }}</option>
            </select>
          </label>
          <label v-if="tables.length > 0" class="g-field">
            <span>Roll on</span>
            <select v-model="draft.table" data-testid="hook-table">
              <option value="">No table: apply an Effect</option>
              <option v-for="t in tables" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
          </label>
          <p v-else class="hint" data-testid="no-tables">Link a Roll Table from your Library to this Campaign to roll on it here.</p>
          <label v-if="draft.table === ''" class="g-field"><span>Effect to apply, by slug</span><input v-model="draft.effect" maxlength="80" data-testid="hook-effect" /></label>
          <GButton type="submit" variant="primary" :disabled="!ready">Add Rule Variant</GButton>
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
.variant {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.variant h2,
.words {
  margin: 0;
}
.variant[data-on='true'] {
  border-color: var(--color-gold-high);
}
.value {
  margin: 0;
  font-weight: 700;
}
.saved {
  margin: 0;
  color: var(--color-gold-high);
}
.hook {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.own {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
