<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getCompendiumGuidesOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { GuideChallenge, GuideRarity, GuideTier, Ruleset } from '@/infrastructure/api/types.gen'
import CompendiumTabs from './CompendiumTabs.vue'

// Guides worked out of the SRD entries: spells by level, the attacks of monsters by Challenge Rating,
// and the loot that suits each tier of play. Open to anyone.
const route = useRoute()
const ruleset = computed(() => (typeof route.query.ruleset === 'string' ? (route.query.ruleset as Ruleset) : undefined))
const within = computed(() => (ruleset.value ? { ruleset: ruleset.value } : {}))
const guides = useQuery(computed(() => ({ ...getCompendiumGuidesOptions({ query: within.value }), retry: false })))

const levels = computed(() => (guides.data.value?.spellsByLevel ?? []).filter((l) => l.spells.length > 0))
const levelName = (level: number) => (level === 0 ? 'Cantrips' : `Level ${String(level)}`)

const signed = (n: number) => (n >= 0 ? `+${String(n)}` : String(n))
function toHit(c: GuideChallenge): string {
  if (c.attacks === 0) return '—'
  return c.toHitLow === c.toHitHigh ? signed(c.toHit) : `${signed(c.toHit)} (${signed(c.toHitLow)} to ${signed(c.toHitHigh)})`
}

const spoken = (rarity: string) => rarity.replace('-', ' ')
const listed = (words: string[]) => (words.length < 2 ? words.join('') : `${words.slice(0, -1).join(', ')} and ${words.at(-1) ?? ''}`)
const tierLine = (t: GuideTier) => `Tier ${String(t.tier)}, levels ${String(t.fromLevel)} to ${String(t.toLevel)}: ${listed(t.rarities.map(spoken))}`
const rarities = computed(() => (guides.data.value?.lootByRarity ?? []).filter((r) => r.items.length > 0))
function rarityLine(r: GuideRarity): string {
  const name = spoken(r.rarity)
  const from = guides.data.value?.lootTiers.find((t) => t.tier === r.firstTier)?.fromLevel ?? 1
  return `${name.charAt(0).toUpperCase()}${name.slice(1)} magic items (${String(r.items.length)}), from level ${String(from)}`
}
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">Compendium</span>
      <h1>Guides</h1>
    </header>
    <CompendiumTabs current="guides" />
    <nav class="rules" aria-label="Ruleset">
      <RouterLink :to="{ name: 'guides' }" :aria-current="ruleset === undefined ? 'page' : undefined">Newest</RouterLink>
      <RouterLink :to="{ name: 'guides', query: { ruleset: 'srd-2024' } }" :aria-current="ruleset === 'srd-2024' ? 'page' : undefined">2024</RouterLink>
      <RouterLink :to="{ name: 'guides', query: { ruleset: 'srd-2014' } }" :aria-current="ruleset === 'srd-2014' ? 'page' : undefined">2014</RouterLink>
    </nav>
    <p v-if="guides.isError.value" role="alert" class="g-alert" data-testid="guides-missing">The guides are not available just now.</p>
    <p v-else-if="guides.isPending.value">Turning the page…</p>
    <template v-else-if="guides.data.value">
      <section class="g-card" aria-labelledby="guide-spells">
        <h2 id="guide-spells">Spells by level</h2>
        <details v-for="l in levels" :key="l.level" data-testid="spell-level">
          <summary>{{ levelName(l.level) }} ({{ l.spells.length }})</summary>
          <ul class="g-list">
            <li v-for="s in l.spells" :key="s.slug">
              <RouterLink :to="{ name: 'spell', params: { slug: s.slug }, query: within }">{{ s.name }}</RouterLink> · {{ s.school }}
            </li>
          </ul>
        </details>
      </section>

      <section class="g-card" aria-labelledby="guide-attacks">
        <h2 id="guide-attacks">Attacks by Challenge Rating</h2>
        <p class="hint">What the attacks of the monsters of each Challenge Rating look like: the middle bonus to hit with the lowest and highest, and the middle damage of one hit on average dice.</p>
        <div class="scroll" role="group" aria-label="Table of attacks by Challenge Rating" tabindex="0">
          <table>
            <thead>
              <tr><th scope="col">Challenge</th><th scope="col">Monsters</th><th scope="col">Attacks</th><th scope="col">To hit</th><th scope="col">Damage of a hit</th></tr>
            </thead>
            <tbody>
              <tr v-for="c in guides.data.value.attacksByChallenge" :key="c.challenge" data-testid="challenge-row">
                <th scope="row">{{ c.challenge }}</th>
                <td>{{ c.monsters }}</td>
                <td>{{ c.attacks }}</td>
                <td>{{ toHit(c) }}</td>
                <td>{{ c.attacks === 0 ? '—' : c.damage }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="g-card" aria-labelledby="guide-loot">
        <h2 id="guide-loot">Loot by party level</h2>
        <p class="hint">The rarities of magic item that suit a party at each tier of play, and the magic items of each rarity.</p>
        <ul class="g-list">
          <li v-for="t in guides.data.value.lootTiers" :key="t.tier" data-testid="loot-tier">{{ tierLine(t) }}</li>
        </ul>
        <details v-for="r in rarities" :key="r.rarity" data-testid="loot-rarity">
          <summary>{{ rarityLine(r) }}</summary>
          <ul class="g-list">
            <li v-for="it in r.items" :key="it.slug">
              <RouterLink :to="{ name: 'entry', params: { kind: 'magic-item', slug: it.slug }, query: within }">{{ it.name }}</RouterLink>
            </li>
          </ul>
        </details>
      </section>
    </template>
  </main>
</template>

<style scoped>
h2 {
  margin: 0 0 8px;
}
.hint {
  margin: 0 0 8px;
  color: var(--color-text-2);
}
.rules {
  display: flex;
  gap: 12px;
}
.rules a,
li a {
  color: var(--color-gold-high);
}
.rules a[aria-current='page'] {
  font-weight: 600;
  text-decoration: none;
}
summary {
  min-height: 44px;
  display: flex;
  align-items: center;
  cursor: pointer;
}
.scroll {
  overflow-x: auto;
}
table {
  border-collapse: collapse;
  width: 100%;
}
th,
td {
  padding: 6px 10px;
  border-bottom: 1px solid var(--color-rule);
  text-align: left;
  white-space: nowrap;
}
</style>
