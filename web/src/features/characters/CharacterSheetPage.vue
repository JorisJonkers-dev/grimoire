<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { deleteCharacterMutation, getCharacterOptions, updateCharacterMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, TokenBadge } from '@/shared/ui'
import PortraitEditor from './PortraitEditor.vue'
import TokenEditor from './TokenEditor.vue'
import { titleCase } from '@/features/compendium/highlight'
import { abbrev, signed } from './build'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const path = computed(() => ({ path: { campaignId: String(route.params.id), characterId: String(route.params.characterId) } }))
const refresh = () => void client.invalidateQueries()
const sheet = useQuery(computed(() => ({ ...getCharacterOptions(path.value), retry: false })))
const s = computed(() => sheet.data.value)
const update = useMutation(updateCharacterMutation())
const remove = useMutation(deleteCharacterMutation())
const failed = ref(false)
const hpPct = computed(() => (s.value ? Math.round((s.value.hpCurrent / s.value.hpMax) * 100) : 0))

function setHp(delta: number) {
  const cur = s.value
  if (!cur) return
  const hp = Math.min(cur.hpMax, Math.max(0, cur.hpCurrent + delta))
  failed.value = false
  update.mutate(
    { ...path.value, body: { hpCurrent: hp } },
    { onSuccess: () => void client.invalidateQueries(), onError: () => (failed.value = true) },
  )
}
function destroy() {
  remove.mutate(path.value, {
    onSuccess: () => void router.push({ name: 'campaign', params: { id: path.value.path.campaignId } }),
    onError: () => (failed.value = true),
  })
}
</script>

<template>
  <main class="g-page sheet">
    <RouterLink :to="{ name: 'campaign', params: { id: path.path.campaignId } }" class="back">← Campaign</RouterLink>
    <p v-if="sheet.isPending.value">Unrolling the sheet…</p>
    <p v-else-if="sheet.isError.value" role="alert" class="g-alert" data-testid="sheet-missing">That character is not in this campaign.</p>
    <article v-else-if="s" data-testid="character-sheet">
      <header class="head">
        <img v-if="s.portraitUrl" :src="s.portraitUrl" :alt="`Portrait of ${s.name}`" class="portrait" data-testid="portrait" />
        <TokenBadge :name="s.name" allegiance="party" :icon-url="s.tokenUrl ?? ''" :size="56" />
        <div>
          <h1>{{ s.name }}</h1>
          <p class="sub">Level {{ s.level }} {{ s.species.name }} {{ s.class.name }} · {{ s.background.name }} · {{ s.ownerName }}</p>
          <p v-if="!s.editable" class="g-tag locked" data-testid="sheet-locked">Read only</p>
        </div>
      </header>
      <PortraitEditor v-if="s.editable" :ids="path.path" @changed="refresh" />

      <section class="vitals" aria-label="Vitals">
        <div class="hp">
          <span class="label">Hit points</span>
          <strong data-testid="hp">{{ s.hpCurrent }} / {{ s.hpMax }}</strong>
          <span class="bar" role="img" :aria-label="`${String(hpPct)} percent`"><span :style="{ width: `${String(hpPct)}%` }" /></span>
          <span v-if="s.editable" class="hp-buttons">
            <GButton data-testid="hp-down" @click="setHp(-1)">−1</GButton>
            <GButton data-testid="hp-up" @click="setHp(1)">+1</GButton>
          </span>
        </div>
        <div class="stat"><span class="label">AC</span><strong data-testid="ac">{{ s.armorClass }}</strong></div>
        <div class="stat"><span class="label">Speed</span><strong>{{ s.speedFeet }} ft</strong></div>
        <div class="stat"><span class="label">Initiative</span><strong>{{ signed(s.initiative) }}</strong></div>
        <div class="stat"><span class="label">Proficiency</span><strong>{{ signed(s.proficiencyBonus) }}</strong></div>
        <div class="stat"><span class="label">Passive Perception</span><strong>{{ s.passivePerception }}</strong></div>
      </section>
      <p v-if="failed" role="alert" class="g-alert">That change was not saved.</p>
      <ul v-if="s.warnings.length" class="warnings">
        <li v-for="w in s.warnings" :key="w">{{ w }}</li>
      </ul>

      <section aria-label="Abilities" class="abilities" data-testid="abilities">
        <div v-for="a in s.abilities" :key="a.ability" class="ability">
          <span class="abbr">{{ abbrev[a.ability] }}</span>
          <strong>{{ signed(a.modifier) }}</strong>
          <span class="score">{{ a.score }}</span>
          <span class="save" :class="{ prof: a.saveProficient }">Save {{ signed(a.save) }}</span>
        </div>
      </section>

      <section class="g-card">
        <h2>Skills</h2>
        <ul class="skills" data-testid="skills">
          <li v-for="k in s.skills" :key="k.skill" :class="{ prof: k.proficient }">
            <span>{{ titleCase(k.skill) }} <small>{{ abbrev[k.ability] }}</small></span><span>{{ signed(k.bonus) }}</span>
          </li>
        </ul>
      </section>

      <section class="g-card">
        <h2>Equipment</h2>
        <p>{{ s.armor?.name ?? 'No armour' }}<template v-if="s.shield"> and a shield</template></p>
        <ul class="g-list">
          <li v-for="w in s.weapons" :key="w.slug">{{ w.name }} · {{ w.damageDice }} {{ w.damageType }}<template v-if="w.longRangeFeet"> · {{ w.rangeFeet }}/{{ w.longRangeFeet }} ft</template></li>
        </ul>
      </section>

      <section class="g-card">
        <h2>Resources</h2>
        <ul class="g-list" data-testid="resources">
          <li v-for="r in s.resources" :key="r.key">{{ r.label }}: {{ r.current }} / {{ r.max }}</li>
        </ul>
        <h2>Active effects</h2>
        <p v-if="s.effects.length === 0" class="hint">None right now.</p>
        <ul v-else class="g-list">
          <li v-for="e in s.effects" :key="e.name">{{ e.name }}: {{ e.detail }}</li>
        </ul>
      </section>

      <TokenEditor
        v-if="s.editable"
        :name="s.name"
        :ids="path.path"
        :portrait-url="s.portraitUrl"
        :token-url="s.tokenUrl"
        @changed="refresh"
      />
      <GButton v-if="s.editable" variant="danger" data-testid="delete-character" @click="destroy()">Delete character</GButton>
    </article>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.portrait {
  width: 96px;
  height: 96px;
  object-fit: cover;
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-bronze);
}
.sub {
  margin: 4px 0 0;
  font-family: var(--font-flavour);
  font-style: italic;
  color: var(--color-text-2);
}
.locked {
  display: inline-block;
  margin-top: 6px;
}
article {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.vitals {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
  gap: 8px;
}
.vitals > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.hp {
  grid-column: 1 / -1;
}
.label {
  font-size: 12px;
  color: var(--color-text-2);
}
strong {
  font-family: var(--font-display);
  font-size: 22px;
}
.bar {
  height: 8px;
  border-radius: var(--radius-pill);
  background: var(--color-raised);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  background: var(--color-success);
}
.hp-buttons {
  display: flex;
  gap: 8px;
}
.warnings {
  margin: 0;
  color: var(--color-enemy-soft);
}
.abilities {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.ability {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 4px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.abbr {
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.score {
  color: var(--color-text-2);
}
.save {
  font-size: 12px;
  color: var(--color-text-2);
}
.save.prof,
.skills li.prof {
  color: var(--color-gold-high);
}
h2 {
  margin: 0 0 8px;
  font-family: var(--font-display);
  font-size: 17px;
}
.skills {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 2px 16px;
}
.skills li {
  display: flex;
  justify-content: space-between;
  min-height: 28px;
}
.hint {
  color: var(--color-text-2);
}
</style>
