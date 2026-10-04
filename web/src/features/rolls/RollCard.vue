<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import type { RollDie, RollRequest } from '@/infrastructure/api/types.gen'
import { keepRoll, rerollDie, rollRest, setDie } from '@/infrastructure/api/sdk.gen'
import { buzz, reducedMotion } from '@/shared/a11y/settings'
import { DieFace, GButton } from '@/shared/ui'
import { criticalOf, shownOf } from '@/features/dice/choreography'
import { throwDice } from '@/features/dice/stage'
import { describeGroup, rollBreakdown, signed } from './notation'

const props = defineProps<{ roll: RollRequest; campaignId: string }>()
const emit = defineEmits<{ updated: [roll: RollRequest] }>()

type Sides = 4 | 6 | 8 | 10 | 12 | 20 | 100
const tumbling = ref<Record<number, number>>({})
// What the player typed for each die they threw on the table.
const typed = ref<Record<number, string>>({})
const failed = ref('')
const busy = ref(false)
const timers = new Set<ReturnType<typeof setInterval>>()
onBeforeUnmount(() => {
  timers.forEach(clearInterval)
})

const pending = computed(() => props.roll.status === 'pending')
const modifierTotal = computed(() => props.roll.modifiers.reduce((sum, m) => sum + m.value, 0))
const dieState = (d: RollDie) =>
  d.no in tumbling.value ? 'rolling' : d.value === undefined ? 'idle' : props.roll.groups[d.group]?.keep ? (d.kept ? 'kept' : 'dropped') : 'idle'
const shown = (d: RollDie) => tumbling.value[d.no] ?? d.value ?? null
const resolved = computed(() => props.roll.status === 'resolved')
// A karmic d20 says what it let go, so nobody has to wonder what the dice did.
const karmic = computed(() => props.roll.dice.filter((d) => d.karmicDropped !== undefined))
// A natural 20 or 1 on the d20 that counts is celebrated once the roll is in.
const natural = computed(() => (resolved.value ? criticalOf(shownOf(props.roll)) : null))
/** The face typed for a die, when it is one the die has. */
function faceOf(d: RollDie): number | null {
  const text = (typed.value[d.no] ?? '').trim()
  const n = Number(text)
  return /^\d+$/.test(text) && n >= 1 && n <= d.faces ? n : null
}
const path = computed(() => ({ campaignId: props.campaignId, rollId: props.roll.id }))

/** Flickers random faces until the server's result lands; with reduced motion it just waits. */
function tumble(dice: RollDie[]): () => void {
  if (reducedMotion()) return () => undefined
  const faces = (d: RollDie) => 1 + Math.floor(Math.random() * d.faces)
  tumbling.value = { ...tumbling.value, ...Object.fromEntries(dice.map((d) => [d.no, faces(d)])) }
  const timer = setInterval(() => {
    tumbling.value = { ...tumbling.value, ...Object.fromEntries(dice.map((d) => [d.no, faces(d)])) }
  }, 70)
  timers.add(timer)
  return () => {
    clearInterval(timer)
    timers.delete(timer)
    const done = new Set(dice.map((d) => String(d.no)))
    tumbling.value = Object.fromEntries(Object.entries(tumbling.value).filter(([no]) => !done.has(no)))
  }
}

async function run(dice: RollDie[], call: () => Promise<RollRequest>) {
  busy.value = true
  failed.value = ''
  const land = tumble(dice)
  try {
    const updated = reducedMotion() ? await call() : (await Promise.all([call(), new Promise((r) => setTimeout(r, 450))]))[0]
    emit('updated', updated)
    // The server has the result: throw it on this screen's dice stage.
    if (updated.status === 'resolved' && !updated.choosing) {
      throwDice(shownOf(updated), updated.id, true)
      buzz(40)
    }
  } catch {
    failed.value = 'That roll could not be saved. Try again.'
  } finally {
    land()
    busy.value = false
  }
}

function auto(d: RollDie) {
  void run([d], async () => (await setDie({ path: { ...path.value, dieNo: d.no }, body: { mode: 'auto' }, throwOnError: true })).data)
}
// Faces typed one after another are sent one after another: a die typed while the last is still being
// saved is kept, not lost.
let entering: Promise<void> = Promise.resolve()
function enter(d: RollDie) {
  const value = faceOf(d)
  if (value === null) return
  typed.value = { ...typed.value, [d.no]: '' }
  const dieNo = d.no
  entering = entering.then(() => run([], async () => (await setDie({ path: { ...path.value, dieNo }, body: { mode: 'manual', value }, throwOnError: true })).data))
}
function keep() {
  void run([], async () => (await keepRoll({ path: path.value, throwOnError: true })).data)
}
function reroll(d: RollDie) {
  void run([d], async () => (await rerollDie({ path: path.value, body: { die: d.no }, throwOnError: true })).data)
}
function rest() {
  const empty = props.roll.dice.filter((d) => d.value === undefined)
  void run(empty, async () => (await rollRest({ path: path.value, throwOnError: true })).data)
}
</script>

<template>
  <article :class="['roll-card', natural ? `roll-card--${natural}` : '']" :aria-label="`Roll: ${roll.purpose}`" data-testid="roll-card">
    <header>
      <h2>{{ roll.purpose }}</h2>
      <p class="who">{{ roll.roller.name }} rolls<template v-if="roll.requestedBy !== roll.roller.name">, asked by {{ roll.requestedBy }}</template></p>
    </header>
    <ul class="throw" aria-label="What to throw">
      <li v-for="g in roll.groups" :key="g.index">{{ describeGroup(g) }}</li>
    </ul>
    <div class="tray">
      <div v-for="d in roll.dice" :key="d.no" class="slot" :data-testid="`die-${String(d.no)}`">
        <DieFace :sides="d.faces as Sides" :value="shown(d)" :state="dieState(d)" :size="72" />
        <span v-if="d.mode" class="mode">{{ d.mode === 'auto' ? 'rolled for you' : 'your die' }}</span>
        <GButton v-if="roll.choosing && roll.mine" :disabled="busy" :data-testid="`reroll-${String(d.no)}`" @click="reroll(d)">Reroll</GButton>
        <template v-else-if="pending && roll.canRoll">
          <form class="entry" :data-testid="`enter-${String(d.no)}`" @submit.prevent="enter(d)">
            <input
              v-model="typed[d.no]"
              class="face"
              type="text"
              inputmode="numeric"
              pattern="[0-9]*"
              autocomplete="off"
              :maxlength="String(d.faces).length"
              :placeholder="`1–${String(d.faces)}`"
              :aria-label="`What your d${String(d.faces)} shows`"
              :data-testid="`face-${String(d.no)}`"
            />
            <GButton type="submit" :disabled="faceOf(d) === null" :data-testid="`set-${String(d.no)}`">Enter</GButton>
          </form>
          <GButton :disabled="busy" :data-testid="`auto-${String(d.no)}`" @click="auto(d)">Roll for me</GButton>
        </template>
      </div>
    </div>
    <GButton v-if="pending && roll.canRoll && roll.dice.some((d) => d.value === undefined)" variant="primary" :disabled="busy" data-testid="roll-rest" @click="rest()">
      Roll the rest for me
    </GButton>
    <div v-if="roll.choosing" class="inspiration" data-testid="inspiration-choice">
      <p v-if="roll.mine">You have Heroic Inspiration: reroll one die and keep the new face, or keep this roll.</p>
      <p v-else>{{ roll.roller.name }} may spend Heroic Inspiration on this roll.</p>
      <GButton v-if="roll.canRoll" variant="primary" :disabled="busy" data-testid="keep-roll" @click="keep()">Keep this roll</GButton>
    </div>
    <p v-if="roll.rerolled" class="who" data-testid="rerolled">Heroic Inspiration spent on a reroll.</p>
    <ul v-if="resolved" class="breakdown" aria-label="What the total is made of" data-testid="roll-breakdown">
      <li v-for="(b, i) in rollBreakdown(roll)" :key="i"><span>{{ b.label }}</span><span>{{ b.value }}</span></li>
    </ul>
    <ul v-else class="breakdown" aria-label="Modifiers">
      <li v-for="m in roll.modifiers" :key="m.label"><span>{{ m.label }}</span><span v-if="m.value !== 0">{{ signed(m.value) }}</span></li>
    </ul>
    <p v-for="d in karmic" :key="d.no" class="who" data-testid="karmic-note">
      Karmic dice: the app rolled {{ d.value }} and {{ d.karmicDropped }} and kept the {{ d.value }}.
    </p>
    <p v-if="natural" class="natural" data-testid="roll-natural">{{ natural === 'hit' ? 'Natural 20!' : 'Natural 1' }}</p>
    <p v-if="roll.status === 'resolved'" class="total" data-testid="roll-total">
      Total <strong>{{ roll.total }}</strong>
      <small v-if="roll.modifiers.length">(modifiers {{ signed(modifierTotal) }})</small>
    </p>
    <p v-else-if="!roll.canRoll && !roll.choosing" class="who">Waiting for {{ roll.roller.name }}…</p>
    <p v-if="failed" role="alert" class="g-alert">{{ failed }}</p>
  </article>
</template>

<style scoped>
.roll-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  border: 1px solid var(--color-gold);
  border-radius: var(--radius-lg);
  background: var(--color-felt);
}
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 19px;
}
.who {
  margin: 2px 0 0;
  color: var(--color-text-2);
}
.throw {
  margin: 0;
  padding-left: 18px;
}
.tray {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.slot {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  min-width: 120px;
}
.mode {
  font-size: 12px;
  color: var(--color-text-2);
}
.entry {
  display: flex;
  gap: 6px;
}
.face {
  width: 76px;
  min-height: 44px;
  padding: 6px 10px;
  border: 1px solid var(--color-bronze);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font-size: 18px;
  text-align: center;
}
.roll-card--hit {
  border-color: var(--color-gold-high);
  box-shadow: 0 0 0 3px rgb(212 175 55 / 35%);
}
.roll-card--miss {
  border-color: var(--color-enemy);
  box-shadow: 0 0 0 3px rgb(200 60 60 / 30%);
}
.natural {
  margin: 0;
  font-family: var(--font-display);
  font-size: 20px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.roll-card--hit .natural {
  color: var(--color-gold-high);
}
.roll-card--miss .natural {
  color: var(--color-enemy-soft);
}
.inspiration {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--color-gold-high);
  border-radius: var(--radius-md);
}
.inspiration p {
  margin: 0;
}
.breakdown {
  margin: 0;
  padding: 0;
  list-style: none;
}
.breakdown li {
  display: flex;
  justify-content: space-between;
  color: var(--color-text-2);
}
.total {
  margin: 0;
  font-size: 18px;
}
.total strong {
  font-family: var(--font-display);
  font-size: 30px;
  color: var(--color-gold-high);
}
</style>
