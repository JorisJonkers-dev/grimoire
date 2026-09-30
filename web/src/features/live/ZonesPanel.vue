<script setup lang="ts">
import type { LiveZone } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { GButton } from '@/shared/ui'

defineProps<{ zones: LiveZone[]; names: Record<string, string> }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()
const status = (z: LiveZone) =>
  z.status === 'sprung' ? 'Sprung' : z.status === 'spotting' ? 'Waiting for Perception' : z.held ? 'Held off' : z.dmOnly ? 'Springs when you say' : 'Armed'
</script>

<template>
  <section class="zones" aria-label="Encounter zones" data-testid="zones">
    <h2>Encounter zones</h2>
    <ul class="g-list">
      <li v-for="z in zones" :key="z.id" class="zone" :data-testid="`zone-${z.name}`">
        <p>
          <strong>{{ z.name }}</strong> · {{ z.radiusHexes }} hex{{ z.radiusHexes === 1 ? '' : 'es' }} · {{ z.creatures }} hidden · {{ status(z) }}<template v-if="z.dc"> · Stealth DC {{ z.dc }}</template>
        </p>
        <ul v-if="z.checks.length > 0" class="checks">
          <li v-for="c in z.checks" :key="c.tokenId">
            {{ names[c.tokenId] ?? 'Someone' }}: {{ c.noticed === undefined ? 'rolling Perception' : c.noticed ? 'noticed' : 'unaware' }}
          </li>
        </ul>
        <div class="row">
          <template v-if="z.status === 'armed'">
            <GButton variant="danger" :aria-label="`Spring ${z.name} now`" @click="emit('send', { kind: 'spring_zone', zoneId: z.id })">Spring it now</GButton>
            <GButton :aria-label="`${z.held ? 'Let' : 'Hold off'} ${z.name}`" @click="emit('send', { kind: 'hold_zone', zoneId: z.id, on: !z.held })">
              {{ z.held ? 'Let it spring' : 'Hold off' }}
            </GButton>
          </template>
          <GButton :aria-label="`Remove ${z.name}`" @click="emit('send', { kind: 'remove_zone', zoneId: z.id })">Remove</GButton>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.zones {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.zone p {
  margin: 0;
}
.checks {
  margin: 4px 0;
  padding-left: 18px;
  color: var(--color-text-2);
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
