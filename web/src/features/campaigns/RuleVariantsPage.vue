<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getCampaignOptions, listRuleVariantsOptions, setRuleVariantsMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { RuleVariant } from '@/infrastructure/api/types.gen'

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
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <h1>Rule Variants</h1>
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
</style>
