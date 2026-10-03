<script setup lang="ts">
import { computed } from 'vue'
import type { LiveRest, LiveToken } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { GButton } from '@/shared/ui'

const props = defineProps<{ rest?: LiveRest; dm: boolean; me: string; tokens: LiveToken[]; inCombat: boolean; hitDiceInLongRest?: boolean }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()

const name = computed(() => (props.rest?.kind === 'long' ? 'Long Rest' : 'Short Rest'))
const canAgree = computed(() => {
  const r = props.rest
  if (!r || r.status !== 'proposed' || r.agreed.includes(props.me)) return false
  return props.dm ? r.waitingOnDm : r.waiting.includes(props.me)
})
const waitingLine = computed(() => {
  const r = props.rest
  if (!r) return ''
  const parts = [...(r.waitingOnDm ? ['the DM'] : []), ...(r.waiting.length ? [`${String(r.waiting.length)} ${r.waiting.length === 1 ? 'player' : 'players'}`] : [])]
  return parts.length ? `Waiting on ${parts.join(' and ')}.` : ''
})
const mayRoll = (tokenId: string) => props.dm || props.tokens.some((t) => t.id === tokenId && t.controllerId === props.me)
</script>

<template>
  <section class="g-card rest" aria-label="Rest" data-testid="rest">
    <h2>Rest</h2>
    <template v-if="!rest">
      <p v-if="inCombat" class="hint">Nobody rests in the middle of a fight.</p>
      <div v-else class="row">
        <GButton data-testid="propose-short" @click="emit('send', { kind: 'propose_rest', rest: 'short' })">Propose a Short Rest</GButton>
        <GButton data-testid="propose-long" @click="emit('send', { kind: 'propose_rest', rest: 'long' })">Propose a Long Rest</GButton>
      </div>
    </template>
    <template v-else>
      <p data-testid="rest-status">
        <strong>{{ name }}</strong> {{ rest.status === 'proposed' ? 'proposed' : 'under way' }}. {{ rest.status === 'proposed' ? waitingLine : '' }}
      </p>
      <GButton v-if="canAgree" variant="primary" data-testid="agree-rest" @click="emit('send', { kind: 'agree_rest' })">Agree to rest</GButton>
      <ul v-if="rest.status === 'resting' && (rest.kind === 'short' || hitDiceInLongRest)" class="g-list">
        <li v-for="r in rest.resters" :key="r.characterId" class="rester" :data-testid="`rester-${r.name}`">
          <span>{{ r.name }} · {{ r.hitDiceLeft }} {{ r.hitDie }} left</span>
          <span v-if="r.rollId" class="hint">Rolling…</span>
          <GButton
            v-else-if="mayRoll(r.tokenId) && r.hitDiceLeft > 0"
            :aria-label="`Spend a Hit Die for ${r.name}`"
            @click="emit('send', { kind: 'spend_hit_die', tokenId: r.tokenId })"
          >
            Spend a Hit Die
          </GButton>
        </li>
      </ul>
      <div v-if="dm" class="row">
        <GButton v-if="rest.status === 'resting'" variant="primary" data-testid="finish-rest" @click="emit('send', { kind: 'finish_rest' })">Finish the rest</GButton>
        <GButton variant="danger" data-testid="interrupt-rest" @click="emit('send', { kind: 'interrupt_rest' })">
          {{ rest.status === 'resting' ? 'Interrupt' : 'Call it off' }}
        </GButton>
      </div>
    </template>
  </section>
</template>

<style scoped>
.rest {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
p {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.rester {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.hint {
  color: var(--color-text-3);
}
</style>
