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
  <main class="spell">
    <RouterLink :to="{ name: 'spells' }" class="back">← All spells</RouterLink>
    <p v-if="spell.isPending.value">Turning the page…</p>
    <p v-else-if="spell.isError.value" role="alert" data-testid="spell-missing">That spell is not in this grimoire.</p>
    <article v-else-if="spell.data.value" data-testid="spell-detail">
      <header>
        <h1>{{ spell.data.value.name }}</h1>
        <p class="sub">
          {{ levelLabel(spell.data.value.level) }} · {{ titleCase(spell.data.value.school) }}
          <template v-if="spell.data.value.ritual"> · ritual</template>
        </p>
        <nav class="rules" aria-label="Ruleset">
          <RouterLink :to="{ name: 'spell', params: { slug }, query: { ruleset: 'srd-2024' } }" :aria-current="spell.data.value.ruleset === 'srd-2024' ? 'page' : undefined">2024</RouterLink>
          <RouterLink :to="{ name: 'spell', params: { slug }, query: { ruleset: 'srd-2014' } }" :aria-current="spell.data.value.ruleset === 'srd-2014' ? 'page' : undefined">2014</RouterLink>
        </nav>
        <CopyLink :to="{ name: 'spell', params: { slug }, query: { ruleset: spell.data.value.ruleset } }" />
      </header>
      <dl class="props">
        <dt>Casting time</dt>
        <dd>{{ spell.data.value.castingTime }}</dd>
        <dt>Range</dt>
        <dd>{{ spell.data.value.rangeText }}</dd>
        <dt>Components</dt>
        <dd>{{ components }}</dd>
        <dt>Duration</dt>
        <dd>{{ spell.data.value.concentration ? 'Concentration, ' : '' }}{{ spell.data.value.duration }}</dd>
        <template v-if="spell.data.value.saveAbility">
          <dt>Save</dt>
          <dd>{{ titleCase(spell.data.value.saveAbility) }}</dd>
        </template>
        <template v-if="spell.data.value.damageRoll">
          <dt>Damage</dt>
          <dd>{{ spell.data.value.damageRoll }} {{ spell.data.value.damageTypes.map(titleCase).join(', ') }}</dd>
        </template>
      </dl>
      <div class="text">
        <p v-for="(para, i) in paragraphs(spell.data.value.description)" :key="i">
          <template v-for="(seg, j) in highlight(para, mentionNames)" :key="j">
            <button v-if="seg.mention" type="button" class="term" :aria-expanded="open?.name === seg.mention" @click="open = byName(seg.mention)">{{ seg.text }}</button>
            <template v-else>{{ seg.text }}</template>
          </template>
        </p>
      </div>
      <section v-if="open" class="popover" role="dialog" :aria-label="open.name" data-testid="condition-popover">
        <h2>{{ open.name }}</h2>
        <p v-for="(para, i) in paragraphs(open.description)" :key="i">{{ para }}</p>
        <button type="button" class="close" @click="open = null">Close</button>
      </section>
      <section v-if="spell.data.value.higherLevel" class="text">
        <h2>At higher levels</h2>
        <p>{{ spell.data.value.higherLevel }}</p>
      </section>
      <p class="classes">{{ spell.data.value.classes.map(titleCase).join(' · ') }}</p>
    </article>
  </main>
</template>

<style scoped>
.spell {
  padding: 24px var(--gutter);
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-sizing: border-box;
  width: 100%;
}
.back {
  color: var(--color-gold-high);
}
h1 {
  margin: 0;
  font-family: var(--font-display);
}
.sub {
  margin: 4px 0 0;
  font-family: var(--font-flavour);
  font-style: italic;
  color: var(--color-text-2);
}
.rules {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
.rules a {
  min-height: 36px;
  padding: 6px 14px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-pill);
  color: var(--color-text);
  text-decoration: none;
}
.rules a[aria-current='page'] {
  border-color: var(--color-gold);
  color: var(--color-gold-high);
}
.props {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 6px 16px;
  margin: 12px 0;
}
dt {
  color: var(--color-text-2);
}
dd {
  margin: 0;
}
.text {
  font-family: var(--font-flavour);
  font-size: 18px;
  line-height: 1.5;
}
.term {
  padding: 0 2px;
  border: 0;
  border-bottom: 1px dashed var(--color-gold-high);
  background: none;
  color: var(--color-gold-high);
  font: inherit;
  cursor: pointer;
}
.popover {
  padding: 14px 16px;
  border: 1px solid var(--color-gold);
  border-radius: var(--radius-lg);
  background: var(--color-raised);
}
.popover h2,
.text h2 {
  margin: 0 0 6px;
  font-family: var(--font-display);
  font-size: 16px;
}
.close {
  min-height: 44px;
  padding: 0 16px;
  border: 1px solid var(--color-bronze);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
}
.classes {
  color: var(--color-text-2);
}
</style>
