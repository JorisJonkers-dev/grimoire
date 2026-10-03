<script setup lang="ts">
import type { LiveLegend, LiveToken } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ token: LiveToken; legend: LiveLegend }>()
const emit = defineEmits<{ send: [command: { kind: 'legendary_action' | 'lair_action' | 'legendary_resistance'; tokenId: string; legend?: string }] }>()
function act(kind: 'legendary_action' | 'lair_action', name: string) {
  emit('send', { kind, tokenId: props.token.id, legend: name })
}
</script>

<template>
  <section class="legend g-card" :aria-label="`${token.label}'s legendary actions`" data-testid="legend-panel">
    <h2>{{ token.label }}</h2>
    <p class="hint" data-testid="legend-left">{{ legend.left }} of {{ legend.uses }} legendary actions left · {{ legend.ready ? 'ready now' : "after the next creature's turn" }}</p>
    <div class="row">
      <GButton
        v-for="a in legend.actions"
        :key="a.name"
        :disabled="!legend.ready || legend.left < a.cost"
        :title="a.text"
        :data-testid="`legendary-${a.name}`"
        @click="act('legendary_action', a.name)"
      >{{ a.name }}<template v-if="a.cost > 1"> ({{ a.cost }})</template></GButton>
    </div>
    <template v-if="legend.lair.length">
      <p class="hint">Lair (initiative 20){{ legend.lairReady ? ' · ready' : '' }}</p>
      <div class="row">
        <GButton v-for="a in legend.lair" :key="a.name" :disabled="!legend.lairReady" :title="a.text" :data-testid="`lair-${a.name}`" @click="act('lair_action', a.name)">{{ a.name }}</GButton>
      </div>
    </template>
    <div class="row">
      <GButton
        :disabled="legend.resistLeft < 1"
        data-testid="legendary-resistance"
        @click="emit('send', { kind: 'legendary_resistance', tokenId: token.id })"
      >Legendary Resistance ({{ legend.resistLeft }})</GButton>
    </div>
    <p v-if="legend.phases" class="hint" data-testid="legend-phase">Phase {{ legend.phase + 1 }} of {{ legend.phases + 1 }}</p>
    <p v-if="legend.threshold" class="hint">Damage threshold {{ legend.threshold }}</p>
  </section>
</template>

<style scoped>
.legend {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
