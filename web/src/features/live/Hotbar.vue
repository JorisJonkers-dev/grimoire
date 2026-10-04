<script setup lang="ts">
import type { LiveSuggestion, LiveToken, Tactics } from '@/infrastructure/api/types.gen'
import { computed, ref } from 'vue'
import { GButton } from '@/shared/ui'
import ActionBars from './ActionBars.vue'
import { actions, areaSpells as spells, damageOf, defaultLayout, masteries, reachOf, signed, summonings, tilesOf, unarmed } from './actionBar'
import { useActionBars } from './useActionBars'

const props = withDefaults(
  defineProps<{
    token: LiveToken
    armed: number | null
    blocked: string
    suggestion?: LiveSuggestion
    target?: string
    tactics?: Tactics
    attacksLeft?: number
    offHand?: boolean
    interaction?: boolean
    cleave?: boolean
    summons?: { tokenId: string; label: string }[]
    own?: boolean
  }>(),
  { suggestion: undefined, target: 'its target', tactics: undefined, attacksLeft: 0, offHand: false, interaction: false, cleave: false, summons: () => [], own: false },
)
const emit = defineEmits<{
  arm: [attackNo: number]
  use: []
  tactics: [value: Tactics]
  area: [effect: string, slot: number]
  action: [action: string]
  ready: [attackNo: number]
  unarmed: [option: string]
  offHand: [attackNo: number]
  cleave: [attackNo: number]
  interact: [what: string]
  swap: []
  teleport: []
  summon: [effect: string]
  jump: []
  ride: []
  throw: []
  command: [tokenId: string]
}>()
const what = ref('')
const slot = ref(0)
const styles: { value: Tactics; label: string }[] = [
  { value: 'auto', label: 'From Intelligence' },
  { value: 'simple', label: 'Simple' },
  { value: 'cunning', label: 'Cunning' },
  { value: 'off', label: 'Off' },
]
const damage = damageOf
const reach = reachOf

// The owner's own layout for this token's Character, once they have arranged one.
const bars = useActionBars(() => props.token.characterId, () => props.own)
const armedKey = computed(() => (props.armed === null ? '' : `attack:${props.token.attacks?.[props.armed]?.name ?? ''}`))
const arranging = ref(false)
function arrangeBars() {
  arranging.value = true
  bars.save(defaultLayout(tilesOf(props.token)))
}
// A tile played from the bars does what its button on the plain hotbar does.
const moves: Record<string, () => void> = {
  swap: () => { emit('swap') },
  jump: () => { emit('jump') },
  mount: () => { emit('ride') },
  throw: () => { emit('throw') },
  'misty-step': () => { emit('teleport') },
}
function play(key: string) {
  const name = key.slice(key.indexOf(':') + 1)
  const plays: Record<string, () => void> = {
    attack: () => { emit('arm', (props.token.attacks ?? []).findIndex((x) => x.name === name)) },
    action: () => { emit('action', name) },
    unarmed: () => { emit('unarmed', name) },
    spell: () => { emit('area', name, slot.value) },
    summon: () => { emit('summon', name) },
    move: () => { moves[name]?.() },
  }
  plays[key.slice(0, key.indexOf(':'))]?.()
}
</script>

<template>
  <section class="hotbar" :aria-label="`${token.label}'s attacks`" :data-testid="`hotbar-${token.label}`">
    <ActionBars
      v-if="bars.layout.value"
      :token="token"
      :layout="bars.layout.value"
      :blocked="blocked"
      :armed="armedKey"
      :start-editing="arranging"
      @use="play"
      @change="bars.save"
    />
    <GButton
      v-for="(a, i) in bars.layout.value ? [] : (token.attacks ?? [])"
      :key="a.name + String(i)"
      :class="['slot', { 'slot--armed': armed === i }]"
      :disabled="blocked !== ''"
      :aria-pressed="armed === i"
      :title="blocked || `${a.name}: ${signed(a.toHit)} to hit, ${damage(a)} ${a.damageType ?? ''}`"
      :data-testid="`attack-${String(i)}`"
      @click="emit('arm', i)"
    >
      <span class="name">{{ a.name }}</span>
      <span class="stat">{{ signed(a.toHit) }} · {{ damage(a) }} · {{ reach(a) }}</span>
      <span v-if="a.mastery" class="g-tag mastery" :title="masteries[a.mastery]" :data-testid="`mastery-${String(i)}`">{{ a.mastery }}</span>
    </GButton>
    <p v-if="attacksLeft" class="hint" data-testid="attacks-left">{{ attacksLeft }} {{ attacksLeft === 1 ? 'attack' : 'attacks' }} left this action</p>
    <GButton
      v-if="cleave"
      variant="primary"
      class="action"
      :disabled="blocked !== ''"
      data-testid="cleave"
      @click="emit('cleave', (token.attacks ?? []).findIndex((a) => a.mastery === 'cleave'))"
    >
      Cleave a second creature
    </GButton>
    <template v-if="offHand">
      <GButton
        v-for="(a, i) in token.attacks ?? []"
        v-show="a.light"
        :key="`off-${a.name}${String(i)}`"
        class="action"
        :disabled="blocked !== ''"
        :data-testid="`off-hand-${String(i)}`"
        @click="emit('offHand', i)"
      >
        Off-hand {{ a.name }}
      </GButton>
    </template>
    <div v-if="suggestion" class="suggestion" data-testid="suggestion">
      <p>
        <strong>Suggested: </strong>
        <template v-if="suggestion.attackNo !== undefined">{{ token.attacks?.[suggestion.attackNo]?.name }} against {{ target }}.</template>
        {{ suggestion.reason }}
      </p>
      <GButton v-if="suggestion.attackNo !== undefined" variant="primary" data-testid="use-suggestion" @click="emit('use')">Use suggestion</GButton>
    </div>
    <div v-if="!bars.layout.value" class="actions" role="group" :aria-label="`${token.label}'s actions`">
      <GButton
        v-for="a in actions"
        :key="a.key"
        class="action"
        :disabled="blocked !== ''"
        :title="a.tip"
        :data-testid="`action-${a.key}`"
        @click="emit('action', a.key)"
      >
        {{ a.name }}
      </GButton>
      <GButton v-for="u in unarmed" :key="u.key" class="action" :disabled="blocked !== ''" :data-testid="`unarmed-${u.key}`" @click="emit('unarmed', u.key)">
        {{ u.name }}
      </GButton>
      <GButton
        v-if="token.kind === 'party'"
        class="action"
        :disabled="blocked !== ''"
        title="Put your weapons away and draw your other set: each costs the equip of an attack or your free interaction; a shield takes your action."
        data-testid="swap-weapons"
        @click="emit('swap')"
      >
        Swap weapons
      </GButton>
    </div>
    <details class="more" data-testid="hotbar-more">
      <summary>More</summary>
      <div class="more-body">
        <form v-if="interaction" class="interact" @submit.prevent="what.trim() && (emit('interact', what.trim()), (what = ''))">
          <label class="g-field grow">
            <span>Free object interaction</span>
            <input v-model="what" maxlength="200" placeholder="draws a dagger" data-testid="interact-what" />
          </label>
          <GButton type="submit" :disabled="blocked !== ''" data-testid="interact">Use</GButton>
        </form>
        <label class="g-field tactics">
          <span>Ready an attack</span>
          <select :disabled="blocked !== ''" data-testid="ready" @change="emit('ready', Number(($event.target as HTMLSelectElement).value))">
            <option value="">When a creature comes within reach…</option>
            <option v-for="(a, i) in token.attacks ?? []" v-show="a.reachFt > 0" :key="a.name + String(i)" :value="i">{{ a.name }}</option>
          </select>
        </label>
        <template v-if="!bars.layout.value">
          <GButton data-testid="jump" title="Leap as far as your Strength score in feet with a run-up." @click="emit('jump')">Jump</GButton>
          <GButton data-testid="ride" title="Get onto a willing creature next to you, or off the one you ride: half your Speed." @click="emit('ride')">{{ token.mountId ? 'Dismount' : 'Mount' }}</GButton>
          <GButton :disabled="blocked !== ''" data-testid="throw" title="Throw the creature you grapple, or a barrel or chest next to you." @click="emit('throw')">Throw</GButton>
          <GButton :disabled="blocked !== ''" data-testid="misty-step" title="Bonus Action: teleport up to 30 feet to a free hex." @click="emit('teleport')">Misty Step</GButton>
        </template>
        <label v-if="!bars.layout.value" class="g-field tactics">
          <span>Area spell</span>
          <select :disabled="blocked !== ''" data-testid="area-spell" @change="emit('area', ($event.target as HTMLSelectElement).value, slot)">
            <option value="">Choose to aim…</option>
            <option v-for="s in spells" :key="s.slug" :value="s.slug">{{ s.name }}</option>
          </select>
        </label>
        <label v-if="!bars.layout.value" class="g-field tactics">
          <span>Summon</span>
          <select :disabled="blocked !== ''" data-testid="summon" @change="emit('summon', ($event.target as HTMLSelectElement).value)">
            <option value="">Choose to place…</option>
            <option v-for="s in summonings" :key="s.slug" :value="s.slug">{{ s.name }}</option>
          </select>
        </label>
        <GButton
          v-for="s in summons"
          :key="s.tokenId"
          :data-testid="`command-${s.label}`"
          title="Bonus Action: command the creature you summoned."
          @click="emit('command', s.tokenId)"
        >
          Command {{ s.label }}
        </GButton>
        <label class="g-field tactics">
          <span>Spell slot</span>
          <select v-model.number="slot" data-testid="area-slot">
            <option :value="0">Lowest</option>
            <option v-for="n in 9" :key="n" :value="n">Level {{ n }}</option>
          </select>
        </label>
        <label v-if="tactics" class="g-field tactics">
          <span>Tactics</span>
          <select :value="tactics" data-testid="tactics" @change="emit('tactics', ($event.target as HTMLSelectElement).value as Tactics)">
            <option v-for="s in styles" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </label>
        <GButton v-if="bars.available.value && !bars.layout.value" data-testid="arrange-bars" @click="arrangeBars">Arrange my bars</GButton>
      </div>
    </details>
    <p v-if="blocked" class="blocked" data-testid="hotbar-blocked">{{ blocked }}</p>
    <p v-else-if="armed !== null" class="hint" role="status">Tap a creature to aim.</p>
  </section>
</template>

<style scoped>
.hotbar {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}
/* A slot is a tile 44 high: the name over what it does. */
.slot {
  display: grid;
  gap: 0;
  min-height: 44px;
  padding: 2px 10px;
  border-color: var(--color-edge);
  text-align: left;
  background: var(--color-raised);
}
.slot--armed {
  border-color: var(--color-brass-edge);
  background: var(--color-selected);
  outline: 2px solid var(--color-gold-high);
  outline-offset: -1px;
}
.name {
  font-family: var(--font-label);
  font-size: 14px;
  line-height: 1.2;
}
.stat,
.blocked,
.hint {
  margin: 0;
  font-family: var(--font-ui);
  font-size: 11px;
  line-height: 1.2;
  color: var(--color-text-2);
}
.blocked,
.hint {
  width: 100%;
  font-size: 12px;
}
.mastery {
  text-transform: capitalize;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  width: 100%;
}
.action,
.more-body :deep(.g-button) {
  min-height: 30px;
  padding: 0 10px;
  font-size: 13px;
}
.suggestion {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  width: 100%;
  font-size: 13px;
}
.suggestion p {
  margin: 0;
}
.suggestion strong {
  margin-right: 4px;
  font-family: var(--font-label);
  font-size: 11px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
.more {
  width: 100%;
}
.more summary {
  display: inline-flex;
  align-items: center;
  min-height: 30px;
  padding: 0 4px;
  font-family: var(--font-label);
  font-size: 13px;
  color: var(--color-gold-high);
  cursor: pointer;
}
.more-body {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: end;
  padding-top: 4px;
}
.interact {
  display: flex;
  gap: 6px;
  align-items: end;
}
.tactics {
  min-width: 150px;
}
@media (pointer: coarse) {
  .action,
  .more summary,
  .more-body :deep(.g-button) {
    min-height: 44px;
  }
}
</style>
