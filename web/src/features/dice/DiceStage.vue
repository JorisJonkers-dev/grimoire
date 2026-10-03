<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { LiveDiceLook } from '@/infrastructure/api/types.gen'
import { DieFace } from '@/shared/ui'
import { GATHER_MS, HOLD_MS, THROW_MS, breakdown, criticalOf, sidesOf, type ShownRoll } from './choreography'
import { goFlat, startWorker, wantsGPU, type DiceWorker } from './gpu'
import { lookFor } from './sets'

// A roll plays whenever `n` changes. The total and the breakdown come straight from the roll the
// server resolved; the 3D dice are only the way there, so whatever they do the result shown is right.
const props = defineProps<{ roll: ShownRoll | null; n: number }>()

const canvas = ref<HTMLCanvasElement>()
const gpu = ref(wantsGPU())
const ready = ref(false)
const phase = ref<'' | 'throw' | 'total'>('')
let worker: DiceWorker | undefined
let timer: ReturnType<typeof setTimeout> | undefined

const critical = computed(() => (props.roll ? criticalOf(props.roll) : null))
const criticalText = computed(() => ({ hit: 'Critical hit', miss: 'Critical miss' } as Record<string, string>)[critical.value ?? ''])

function flat() {
  gpu.value = false
  goFlat()
  worker?.stop()
  worker = undefined
  // A roll caught mid-throw skips to its total.
  if (phase.value === 'throw') total()
}
function total() {
  clearTimeout(timer)
  phase.value = 'total'
  timer = setTimeout(() => {
    phase.value = ''
    worker?.post({ type: 'clear' })
  }, HOLD_MS)
}
function size() {
  return { width: window.innerWidth, height: window.innerHeight, dpr: window.devicePixelRatio || 1 }
}
function resize() {
  worker?.post({ type: 'size', ...size() })
}
function play() {
  if (!props.roll) return
  clearTimeout(timer)
  if (!gpu.value || !ready.value || !worker) {
    total()
    return
  }
  phase.value = 'throw'
  // The total shows on time even if the worker never answers.
  timer = setTimeout(total, THROW_MS + GATHER_MS)
  try {
    // The dice are copied plain: a roll that came through the page's state cannot cross to a worker as it is.
    const dice = props.roll.dice.map((d) => ({ faces: d.faces, value: d.value, kept: d.kept }))
    const look = props.roll.look ? { look: JSON.parse(JSON.stringify(props.roll.look)) as LiveDiceLook } : {}
    worker.post({ type: 'roll', dice, seed: props.n, ...look })
  } catch {
    flat()
  }
}
watch(() => props.n, play)

onMounted(() => {
  if (!gpu.value || !canvas.value) return
  try {
    const offscreen = canvas.value.transferControlToOffscreen()
    worker = startWorker((m) => {
      if (m.type === 'ready') ready.value = true
      else if (m.type === 'settled') {
        if (phase.value === 'throw') total()
      } else flat()
    })
    worker.post({ type: 'init', canvas: offscreen, ...size() }, [offscreen])
    window.addEventListener('resize', resize)
  } catch {
    flat()
  }
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  window.removeEventListener('resize', resize)
  worker?.stop()
})
</script>

<template>
  <div class="stage" data-testid="dice-stage">
    <canvas v-if="gpu" ref="canvas" class="canvas" aria-hidden="true" data-testid="dice-canvas"></canvas>
    <div v-if="roll && phase === 'total'" :class="['result', critical ? `result--${critical}` : '']" role="status" data-testid="dice-result">
      <p class="who">{{ roll.roller }} · {{ roll.purpose }}</p>
      <div v-if="!gpu" class="flat" data-testid="dice-2d">
        <DieFace v-for="(d, i) in roll.dice" :key="i" :sides="sidesOf(d.faces)" :value="d.value" :state="d.kept ? 'kept' : 'dropped'" :size="64" :look="lookFor(roll.look, d.faces)" />
      </div>
      <p class="total" data-testid="dice-total">{{ roll.total }}</p>
      <p v-if="criticalText" class="critical" data-testid="dice-critical">{{ criticalText }}</p>
      <p class="breakdown" data-testid="dice-breakdown">{{ breakdown(roll) }}</p>
    </div>
  </div>
</template>

<style scoped>
.stage {
  position: fixed;
  inset: 0;
  z-index: 6;
  display: grid;
  place-items: center;
  pointer-events: none;
}
.canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}
.result {
  position: relative;
  display: grid;
  justify-items: center;
  gap: 4px;
  margin-top: 22vh;
  padding: 14px 32px;
  border: 2px solid var(--color-line);
  border-radius: 16px;
  color: var(--color-text);
  background: color-mix(in srgb, var(--color-surface) 92%, transparent);
  box-shadow: 0 12px 40px rgb(0 0 0 / 50%);
  animation: land 260ms ease-out;
}
.result--hit {
  border-color: var(--color-gold-high);
  box-shadow: 0 0 0 4px rgb(212 175 55 / 35%), 0 12px 40px rgb(0 0 0 / 50%);
}
.result--miss {
  border-color: var(--color-enemy);
  box-shadow: 0 0 0 4px rgb(200 60 60 / 30%), 0 12px 40px rgb(0 0 0 / 50%);
}
.who,
.breakdown,
.critical {
  margin: 0;
}
.who,
.breakdown {
  color: var(--color-text-2);
}
.flat {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
}
.total {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(56px, 12vw, 128px);
  font-weight: 700;
  line-height: 1;
}
.result--hit .total,
.result--hit .critical {
  color: var(--color-gold-high);
}
.result--miss .total,
.result--miss .critical {
  color: var(--color-enemy-soft);
}
.critical {
  font-family: var(--font-display);
  font-size: 20px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
@keyframes land {
  from {
    transform: scale(0.6);
  }
}
@media (prefers-reduced-motion: reduce) {
  .result {
    animation: none;
  }
}
</style>
