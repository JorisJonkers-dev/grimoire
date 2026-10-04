<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch, type ComponentPublicInstance } from 'vue'
import type { LiveCombat, LiveInitiativeRoll, LiveRosterEntry, LiveToken } from '@/infrastructure/api/types.gen'
import { StatusIcon } from '@/shared/ui'
import { initials } from './board'
import { effectLabel } from './conditions'
import { reducedMotion } from '@/shared/a11y/settings'
import { REVEAL_FADE_MS, REVEAL_HOLD_MS } from './motion'

const props = withDefaults(
  defineProps<{ roster: LiveRosterEntry[]; combat?: LiveCombat; tokens?: LiveToken[]; reveal?: { order: LiveInitiativeRoll[]; n: number } | null }>(),
  { combat: undefined, tokens: () => [], reveal: null },
)
const emit = defineEmits<{ effects: [tokenId: string] }>()

const combatantOf = (tokenId: string) => props.combat?.combatants.find((c) => c.tokenId === tokenId)
const dyingOf = (tokenId: string) => {
  const d = props.tokens.find((t) => t.id === tokenId)?.dying
  if (!d) return ''
  if (d.dead) return 'Dead'
  return d.stable ? 'Stable' : `Dying ${String(d.successes)}✓ ${String(d.failures)}✗`
}
const ownerName = (id: string) => props.combat?.combatants.find((c) => c.id === id)?.label ?? 'someone'
const tied = computed(() => {
  const ranks = props.combat?.combatants.map((c) => c.rank) ?? []
  return (rank?: number) => ranks.filter((r) => r === rank).length > 1
})
// How full a creature's bar is: its hit points when the audience may know them, or what anyone can tell.
const bands: Record<string, number> = { unhurt: 100, hurt: 70, bloodied: 40, down: 0 }
function fill(e: LiveRosterEntry): number {
  if (e.hp !== undefined && e.hpMax) return Math.round((Math.max(0, e.hp) * 100) / e.hpMax)
  return bands[e.health ?? 'unhurt'] ?? 100
}
function healthText(e: LiveRosterEntry): string {
  if (e.hp !== undefined && e.hpMax !== undefined) return `${String(e.hp)} of ${String(e.hpMax)} hit points${e.tempHp ? ` and ${String(e.tempHp)} temporary` : ''}`
  return e.health ?? 'unhurt'
}

// As a fight begins the rolls show over the faces where they stood, the faces slide into initiative
// order, and the numbers fade. Reduced motion goes straight to the order.
const phase = ref<'' | 'rolls' | 'settle'>('')
let stood: string[] = []
const held = ref<string[]>([])
let timer: ReturnType<typeof setTimeout> | undefined
watch(() => props.roster, (_now, before) => { stood = before.map((e) => e.tokenId) })
watch(() => props.reveal?.n, (n) => {
  if (n === undefined || reducedMotion()) return
  clearTimeout(timer)
  held.value = stood
  phase.value = 'rolls'
  timer = setTimeout(() => {
    phase.value = 'settle'
    timer = setTimeout(() => { phase.value = '' }, REVEAL_FADE_MS)
  }, REVEAL_HOLD_MS)
})
onBeforeUnmount(() => { clearTimeout(timer) })
const place = (id: string) => (held.value.includes(id) ? held.value.indexOf(id) : held.value.length)
const shown = computed(() => (phase.value === 'rolls' ? [...props.roster].sort((a, b) => place(a.tokenId) - place(b.tokenId)) : props.roster))
const rolled = (tokenId: string) => (phase.value ? props.reveal?.order.find((r) => r.tokenId === tokenId)?.initiative : undefined)

// The strip keeps the face that acts now in view.
const strip = ref<ComponentPublicInstance>()
const acting = computed(() => props.roster.find((e) => e.acting)?.tokenId)
watch(acting, async (id) => {
  await nextTick()
  const el = id ? (strip.value?.$el as HTMLElement | undefined)?.querySelector<HTMLElement>(`[data-token="${id}"]`) : undefined
  el?.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' })
})
</script>

<template>
  <section :class="['roster', { 'roster--reveal': phase, 'roster--settle': phase === 'settle' }]" aria-label="Roster" data-testid="roster-strip">
    <h2 v-if="combat" class="round" data-testid="initiative-rail">{{ combat.status === 'rolling' ? 'Rolling initiative' : `Round ${String(combat.round)}` }}</h2>
    <!-- The strip scrolls sideways when it is wider than the screen, so the keyboard has to be able to reach it. -->
    <TransitionGroup ref="strip" tag="ol" name="slot" tabindex="0" aria-label="Creatures, in order">
      <li
        v-for="e in shown"
        :key="e.tokenId"
        :class="['slot', `slot--${e.kind}`, { 'slot--acting': e.acting, 'slot--done': combatantOf(e.tokenId)?.done, 'slot--hidden': e.hidden }]"
        :aria-current="e.acting ? 'step' : undefined"
        :data-token="e.tokenId"
        :data-testid="`rail-${e.label}`"
      >
        <span v-if="rolled(e.tokenId) !== undefined" class="rolled" :data-testid="`rolled-${e.label}`">{{ rolled(e.tokenId) }}</span>
        <span class="badge" aria-hidden="true">{{ initials(e.label) }}</span>
        <span class="name">{{ e.label }}<span v-if="e.hidden" class="sr-only"> (hidden)</span></span>
        <span v-if="e.companion" class="note" :data-testid="`companion-${e.label}`">Companion</span>
        <span class="bar" role="img" :aria-label="healthText(e)" :data-testid="`health-${e.label}`">
          <span :class="['fill', `fill--${e.health ?? 'known'}`]" :style="{ width: `${String(fill(e))}%` }" />
        </span>
        <template v-if="combatantOf(e.tokenId)">
          <span class="init">
            <template v-if="combatantOf(e.tokenId)?.initiative !== undefined">{{ combatantOf(e.tokenId)?.initiative }}<template v-if="tied(combatantOf(e.tokenId)?.rank)"> · tied</template></template>
            <template v-else>rolling…</template>
          </span>
          <span v-if="combatantOf(e.tokenId)?.surprised" class="note" data-testid="surprised">Surprised</span>
          <span v-if="combatantOf(e.tokenId)?.ownerId" class="summoned" :data-testid="`summoned-${e.label}`">
            Summoned by {{ ownerName(combatantOf(e.tokenId)?.ownerId ?? '') }}<template v-if="combatantOf(e.tokenId)?.awaitingCommand"> · awaiting orders</template>
          </span>
        </template>
        <span v-if="dyingOf(e.tokenId)" class="note" data-testid="fallen">{{ dyingOf(e.tokenId) }}</span>
        <button
          v-if="e.effects.length"
          type="button"
          class="statuses"
          :aria-label="`Effects on ${e.label}: ${e.effects.map((x) => effectLabel(x)).join(', ')}`"
          data-testid="statuses"
          @click="emit('effects', e.tokenId)"
        >
          <StatusIcon v-for="x in e.effects" :key="x.id" :slug="x.slug" :label="effectLabel(x)" :size="12" :icon="x.icon" :color="x.color" />
        </button>
        <span v-if="e.acting" class="sr-only">acting now</span>
      </li>
    </TransitionGroup>
  </section>
</template>

<style scoped>
.roster {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  margin: 0 auto;
}
.round {
  margin: 0;
  font-size: 14px;
  color: var(--color-text-2);
}
ol:focus-visible {
  outline: 2px solid var(--color-gold-high);
  outline-offset: 2px;
}
ol {
  display: flex;
  gap: 6px;
  max-width: 100%;
  margin: 0;
  padding: 4px 4px 6px;
  overflow-x: auto;
  scroll-snap-type: x proximity;
  list-style: none;
}
.slot {
  display: grid;
  justify-items: center;
  gap: 2px;
  min-width: 76px;
  padding: 6px;
  border: 2px solid var(--color-line);
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-surface) 82%, transparent);
  backdrop-filter: blur(6px);
  scroll-snap-align: center;
}
.slot-move {
  transition: transform 600ms ease;
}
.rolled {
  max-height: 32px;
  font-family: var(--font-display);
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
  color: var(--color-gold-high);
}
.roster--settle .rolled {
  animation: roll-fade 1600ms ease forwards;
}
/* The number shrinks away rather than fading: half-faded text would be too faint to read. */
@keyframes roll-fade {
  to {
    max-height: 0;
    transform: scale(0);
  }
}
@media (prefers-reduced-motion: reduce) {
  .slot-move {
    transition: none;
  }
  .roster--settle .rolled {
    animation: none;
  }
}
.slot--acting {
  border-color: var(--color-gold-high);
  box-shadow: 0 0 0 2px rgb(212 175 55 / 35%);
}
.slot--done {
  opacity: 0.6;
}
.slot--hidden {
  border-style: dashed;
}
.badge {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  font-family: var(--font-display);
  font-weight: 700;
  background: var(--color-enemy-fill);
  border: 2px solid var(--color-enemy);
}
.slot--party .badge {
  background: var(--color-party-fill);
  border-color: var(--color-party);
}
.name {
  max-width: 88px;
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bar {
  width: 64px;
  height: 5px;
  overflow: hidden;
  border-radius: 3px;
  background: rgb(255 255 255 / 12%);
}
.fill {
  display: block;
  height: 100%;
  background: var(--color-success);
}
.fill--bloodied,
.fill--down {
  background: var(--color-enemy);
}
.fill--hurt {
  background: var(--color-gold-high);
}
.init,
.summoned {
  font-size: 11px;
  color: var(--color-text-2);
}
.note {
  font-size: 12px;
  color: var(--color-enemy-soft);
}
.statuses {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 2px;
  min-width: 44px;
  min-height: 24px;
  padding: 2px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
}
.statuses:focus-visible {
  outline: 2px solid var(--color-gold-high);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
