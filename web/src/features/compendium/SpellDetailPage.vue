<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getSpellOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { ConditionRef, Ruleset } from '@/infrastructure/api/types.gen'
import CopyLink from './CopyLink.vue'
import { highlight, levelLabel, titleCase } from './highlight'

const route = useRoute()
const slug = computed(() => String(route.params.slug))
const ruleset = computed(() => (typeof route.query.ruleset === 'string' ? (route.query.ruleset as Ruleset) : undefined))
const spell = useQuery(
  computed(() => ({
    ...getSpellOptions({ path: { slug: slug.value }, query: ruleset.value ? { ruleset: ruleset.value } : {} }),
    retry: false,
  })),
)
const open = ref<ConditionRef | null>(null)
const mentionNames = computed(() => spell.data.value?.mentions.map((m) => m.name) ?? [])
const byName = (name: string) => spell.data.value?.mentions.find((m) => m.name === name) ?? null
const components = computed(() => {
  const s = spell.data.value
  if (!s) return ''
  const parts = [s.verbal ? 'V' : '', s.somatic ? 'S' : '', s.material ? 'M' : ''].filter(Boolean).join(', ')
  return s.materialText ? `${parts} (${s.materialText})` : parts
})
const paragraphs = (text: string) => text.split(/\n+/).filter((p) => p.trim() !== '')
</script>

<template>
  <main class="g-page">
    <nav aria-label="Breadcrumb" class="g-crumbs">
      <RouterLink :to="{ name: 'spells' }">Compendium</RouterLink> <span aria-hidden="true">›</span>
      <RouterLink :to="{ name: 'spells' }" class="back">Spells</RouterLink> <span aria-hidden="true">›</span>
      <span aria-current="page">{{ spell.data.value?.name ?? slug }}</span>
    </nav>
    <p v-if="spell.isPending.value">Turning the page…</p>
    <p v-else-if="spell.isError.value" role="alert" data-testid="spell-missing">That spell is not in this grimoire.</p>
    <article v-else-if="spell.data.value" class="body" data-testid="spell-detail">
      <header class="g-detail-head">
        <div>
          <span class="g-eyebrow">Spell · SRD {{ spell.data.value.ruleset === 'srd-2024' ? '5.2' : '5.1' }}</span>
          <h1>{{ spell.data.value.name }}</h1>
          <p class="g-meta sub">
            <span>{{ levelLabel(spell.data.value.level) }} · {{ titleCase(spell.data.value.school) }}<template v-if="spell.data.value.ritual"> · ritual</template></span>
            <span class="classes">{{ spell.data.value.classes.map(titleCase).join(' · ') }}</span>
          </p>
        </div>
        <div class="g-acts">
          <nav class="g-segment" aria-label="Ruleset">
            <RouterLink :to="{ name: 'spell', params: { slug }, query: { ruleset: 'srd-2024' } }" :aria-current="spell.data.value.ruleset === 'srd-2024' ? 'page' : undefined">2024</RouterLink>
            <RouterLink :to="{ name: 'spell', params: { slug }, query: { ruleset: 'srd-2014' } }" :aria-current="spell.data.value.ruleset === 'srd-2014' ? 'page' : undefined">2014</RouterLink>
          </nav>
          <CopyLink :to="{ name: 'spell', params: { slug }, query: { ruleset: spell.data.value.ruleset } }" />
        </div>
      </header>
      <dl class="g-facts props">
        <div>
          <dt>Casting time</dt>
          <dd>{{ spell.data.value.castingTime }}</dd>
        </div>
        <div>
          <dt>Range</dt>
          <dd>{{ spell.data.value.rangeText }}</dd>
        </div>
        <div>
          <dt>Components</dt>
          <dd>{{ components }}</dd>
        </div>
        <div>
          <dt>Duration</dt>
          <dd>{{ spell.data.value.concentration ? 'Concentration, ' : '' }}{{ spell.data.value.duration }}</dd>
        </div>
        <div v-if="spell.data.value.saveAbility">
          <dt>Save</dt>
          <dd>{{ titleCase(spell.data.value.saveAbility) }}</dd>
        </div>
        <div v-if="spell.data.value.damageRoll">
          <dt>Damage</dt>
          <dd>{{ spell.data.value.damageRoll }} {{ spell.data.value.damageTypes.map(titleCase).join(', ') }}</dd>
        </div>
      </dl>
      <section class="part" aria-labelledby="spell-description">
        <h2 id="spell-description">Description</h2>
        <div class="g-prose text">
          <p v-for="(para, i) in paragraphs(spell.data.value.description)" :key="i">
            <template v-for="(seg, j) in highlight(para, mentionNames)" :key="j">
              <button v-if="seg.mention" type="button" class="term" :aria-expanded="open?.name === seg.mention" @click="open = byName(seg.mention)">{{ seg.text }}</button>
              <template v-else>{{ seg.text }}</template>
            </template>
          </p>
        </div>
      </section>
      <section v-if="open" class="popover" role="dialog" :aria-label="open.name" data-testid="condition-popover">
        <h3>{{ open.name }}</h3>
        <p v-for="(para, i) in paragraphs(open.description)" :key="i">{{ para }}</p>
        <button type="button" class="g-action close" @click="open = null">Close</button>
      </section>
      <section v-if="spell.data.value.higherLevel" class="part" aria-labelledby="spell-higher">
        <h2 id="spell-higher">At higher levels</h2>
        <div class="g-prose text">
          <p>{{ spell.data.value.higherLevel }}</p>
        </div>
      </section>
    </article>
  </main>
</template>

<style scoped>
.g-page {
  gap: 32px;
  padding-top: 40px;
}
.body {
  display: flex;
  flex-direction: column;
  gap: 36px;
  max-width: 980px;
}
.part {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
/* A word the rules define is marked; pressing it shows what it means. */
.term {
  padding: 0;
  border: 0;
  border-bottom: 1px solid var(--color-brass-line);
  background: none;
  color: var(--color-gold-high);
  font: inherit;
  cursor: pointer;
}
.popover {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px 18px;
  border: 1px solid var(--color-edge);
  border-radius: var(--radius-panel);
  background: var(--color-surface);
  box-shadow: 0 14px 32px rgb(0 0 0 / 55%);
}
.popover p {
  margin: 0;
  font-size: 15px;
  color: var(--color-text-2);
}
.close {
  align-self: flex-start;
  min-height: var(--size-touch-target);
}
</style>
