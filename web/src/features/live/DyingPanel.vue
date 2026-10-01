<script setup lang="ts">
import { computed } from 'vue'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { GButton } from '@/shared/ui'

const props = defineProps<{ tokens: LiveToken[]; dm: boolean; helper: LiveToken | null }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()
const down = computed(() => props.tokens.filter((t) => t.dying))
const status = (t: LiveToken) => {
  const d = t.dying
  if (!d) return ''
  if (d.dead) return 'Dead'
  if (d.stable) return 'Stable'
  return `${String(d.successes)} ${d.successes === 1 ? 'success' : 'successes'}, ${String(d.failures)} ${d.failures === 1 ? 'failure' : 'failures'}`
}
</script>

<template>
  <section v-if="down.length" class="g-card dying" aria-label="The fallen" data-testid="dying">
    <h2>The fallen</h2>
    <ul class="g-list">
      <li v-for="t in down" :key="t.id" :data-testid="`dying-${t.label}`">
        <span class="who">{{ t.label }}: {{ status(t) }}</span>
        <span v-if="t.dying && !t.dying.dead && !t.dying.stable" class="pips" aria-hidden="true">
          <span v-for="n in 3" :key="`s${String(n)}`" :class="['pip', 'pip--save', { on: n <= (t.dying?.successes ?? 0) }]" />
          <span v-for="n in 3" :key="`f${String(n)}`" :class="['pip', 'pip--fail', { on: n <= (t.dying?.failures ?? 0) }]" />
        </span>
        <template v-if="t.dying && !t.dying.dead && !t.dying.stable && helper && helper.id !== t.id">
          <GButton :aria-label="`Stabilise ${t.label} with Medicine`" @click="emit('send', { kind: 'stabilise', tokenId: helper.id, targetId: t.id, option: 'medicine' })">
            Medicine
          </GButton>
          <GButton :aria-label="`Stabilise ${t.label} with a spell`" @click="emit('send', { kind: 'stabilise', tokenId: helper.id, targetId: t.id, option: 'spell' })">
            Spare the Dying
          </GButton>
        </template>
        <template v-if="t.dying?.dead && dm">
          <GButton
            v-for="s in ['revivify', 'raise_dead', 'resurrection'] as const"
            :key="s"
            :aria-label="`Revive ${t.label} with ${s.replace('_', ' ')}`"
            @click="emit('send', { kind: 'revive', targetId: t.id, option: s })"
          >
            {{ s.replace('_', ' ') }}
          </GButton>
        </template>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.dying {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.pips {
  display: inline-flex;
  gap: 3px;
}
.pip {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  border: 1px solid var(--color-line);
}
.pip--save.on {
  background: var(--color-success);
}
.pip--fail.on {
  background: var(--color-enemy);
}
button {
  text-transform: capitalize;
}
</style>
