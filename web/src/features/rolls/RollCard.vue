<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import type { RollDie, RollRequest } from '@/infrastructure/api/types.gen'
import { keepRoll, rerollDie, rollRest, setDie } from '@/infrastructure/api/sdk.gen'
import { DieFace, GButton } from '@/shared/ui'
import { describeGroup, signed } from './notation'

const props = defineProps<{ roll: RollRequest; campaignId: string }>()
const emit = defineEmits<{ updated: [roll: RollRequest] }>()

type Sides = 4 | 6 | 8 | 10 | 12 | 20 | 100
const tumbling = ref<Record<number, number>>({})
const padFor = ref<number | null>(null)
const failed = ref('')
const busy = ref(false)
const timers = new Set<ReturnType<typeof setInterval>>()
onBeforeUnmount(() => {
  timers.forEach(clearInterval)
})

const reduced = () => typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
const pending = computed(() => props.roll.status === 'pending')
const modifierTotal = computed(() => props.roll.modifiers.reduce((sum, m) => sum + m.value, 0))
const dieState = (d: RollDie) =>
  d.no in tumbling.value ? 'rolling' : d.value === undefined ? 'idle' : props.roll.groups[d.group]?.keep ? (d.kept ? 'kept' : 'dropped') : 'idle'
const shown = (d: RollDie) => tumbling.value[d.no] ?? d.value ?? null
const path = computed(() => ({ campaignId: props.campaignId, rollId: props.roll.id }))

/** Flickers random faces until the server's result lands; with reduced motion it just waits. */
function tumble(dice: RollDie[]): () => void {
  if (reduced()) return () => undefined
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
    const updated = reduced() ? await call() : (await Promise.all([call(), new Promise((r) => setTimeout(r, 450))]))[0]
    emit('updated', updated)
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
function enter(d: RollDie, value: number) {
  padFor.value = null
  void run([], async () => (await setDie({ path: { ...path.value, dieNo: d.no }, body: { mode: 'manual', value }, throwOnError: true })).data)
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
  <article class="roll-card" :aria-label="`Roll: ${roll.purpose}`" data-testid="roll-card">
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
          <GButton :disabled="busy" :data-testid="`auto-${String(d.no)}`" @click="auto(d)">Roll for me</GButton>
          <GButton :disabled="busy" :aria-expanded="padFor === d.no" :data-testid="`manual-${String(d.no)}`" @click="padFor = padFor === d.no ? null : d.no">
            I rolled it
          </GButton>
          <div v-if="padFor === d.no" class="pad" role="group" :aria-label="`Face of the d${String(d.faces)}`" :data-testid="`pad-${String(d.no)}`">
            <button v-for="n in d.faces" :key="n" type="button" @click="enter(d, n)">{{ n }}</button>
          </div>
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
    <ul class="breakdown" aria-label="Modifiers">
      <li v-for="m in roll.modifiers" :key="m.label"><span>{{ m.label }}</span><span>{{ signed(m.value) }}</span></li>
    </ul>
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
.pad {
  display: grid;
  grid-template-columns: repeat(5, 44px);
  gap: 4px;
  max-height: 200px;
  overflow-y: auto;
}
.pad button {
  min-height: 44px;
  border: 1px solid var(--color-bronze);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font-size: 16px;
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
