<script setup lang="ts">
import { ref, watch } from 'vue'
import type { LiveMarcher } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { GButton } from '@/shared/ui'
import { clockText, nextDawn } from './travel'

const DAY_MINUTES = 24 * 60

const props = defineProps<{ gameDay: number; gameMinute: number; marchingOrder: LiveMarcher[]; dm: boolean }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()

// The DM moves the Game Clock on by an hour or to the next dawn, or sets a day and a time.
const day = ref(props.gameDay)
const time = ref('06:00')
watch(() => props.gameDay, (d) => { day.value = d })
const set = (gameDay: number, gameMinute: number) => { emit('send', { kind: 'set_clock', gameDay, gameMinute }) }
function hourOn() {
  const total = props.gameDay * DAY_MINUTES + props.gameMinute + 60
  set(Math.floor(total / DAY_MINUTES), total % DAY_MINUTES)
}
function toDawn() {
  const dawn = nextDawn(props.gameDay, props.gameMinute)
  set(dawn.gameDay, dawn.gameMinute)
}
function setTo() {
  set(day.value, Number(time.value.slice(0, 2)) * 60 + Number(time.value.slice(3, 5)))
}

// Anyone at the table arranges the Marching Order: moving a Character sends the whole order anew.
function move(at: number, by: number) {
  const ids = props.marchingOrder.map((m) => m.characterId)
  ids.splice(at + by, 0, ...ids.splice(at, 1))
  emit('send', { kind: 'set_marching_order', characterIds: ids })
}
</script>

<template>
  <section class="g-card clock" aria-label="Game Clock and Marching Order">
    <p class="now" role="status"><span class="sr-only">Game Clock: </span><span data-testid="game-clock">{{ clockText(gameDay, gameMinute) }}</span></p>
    <div v-if="dm" class="row" data-testid="clock-tools">
      <GButton data-testid="clock-hour" @click="hourOn()">An hour on</GButton>
      <GButton data-testid="clock-dawn" @click="toDawn()">To the next dawn</GButton>
      <label class="g-field"><span>Day</span><input v-model.number="day" type="number" min="0" max="1000000" data-testid="clock-day" /></label>
      <label class="g-field"><span>Time</span><input v-model="time" type="time" data-testid="clock-time" /></label>
      <GButton data-testid="clock-set" @click="setTo()">Set the clock</GButton>
    </div>
    <h2>Marching order</h2>
    <p class="hint">Who goes in front meets a trap or an ambush first.</p>
    <ol class="g-list" data-testid="marching-order">
      <li v-for="(m, i) in marchingOrder" :key="m.characterId" class="row">
        <span class="who" data-testid="marcher">{{ m.place ? `${String(m.place)}. ${m.name}` : `${m.name}, not placed` }}</span>
        <GButton :disabled="i === 0" :aria-label="`Move ${m.name} towards the front`" :data-testid="`march-up-${m.characterId}`" @click="move(i, -1)">↑</GButton>
        <GButton :disabled="i === marchingOrder.length - 1" :aria-label="`Move ${m.name} towards the back`" :data-testid="`march-down-${m.characterId}`" @click="move(i, 1)">↓</GButton>
      </li>
      <li v-if="marchingOrder.length === 0" class="hint">No Characters in this Campaign yet.</li>
    </ol>
  </section>
</template>

<style scoped>
.clock {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.now {
  margin: 0;
  font-weight: 700;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.who {
  flex: 1;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
h2 {
  margin: 0;
}
</style>
