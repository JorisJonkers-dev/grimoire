<script setup lang="ts">
import { computed, ref } from 'vue'
import type { EncounterMode, EncounterTable, LiveCheck } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { GButton } from '@/shared/ui'
import { checkLine } from './checks'

const props = defineProps<{ checks: LiveCheck[]; dm: boolean; tables: EncounterTable[] }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()
const tableId = ref('')
const mode = ref<EncounterMode>('normal')
const entry = ref(0)
const due = ref<'next_rest' | 'next_travel'>('next_rest')
const chosen = computed(() => props.tables.find((t) => t.id === tableId.value))
const recent = computed(() => [...props.checks].reverse())
</script>

<template>
  <section class="g-card checks" aria-label="Encounter checks" data-testid="encounter-checks">
    <h2>Encounter checks</h2>
    <template v-if="dm">
      <div class="row">
        <GButton data-testid="rest-short" @click="emit('send', { kind: 'rest', rest: 'short' })">Short rest</GButton>
        <GButton data-testid="rest-long" @click="emit('send', { kind: 'rest', rest: 'long' })">Long rest</GButton>
      </div>
      <div class="row">
        <label class="g-field grow">
          <span>Table</span>
          <select v-model="tableId" data-testid="check-table">
            <option value="" disabled>Choose a table</option>
            <option v-for="t in tables" :key="t.id" :value="t.id">{{ t.name }}</option>
          </select>
        </label>
        <label class="g-field">
          <span>How</span>
          <select v-model="mode" data-testid="check-mode">
            <option value="normal">Roll the chance</option>
            <option value="force_encounter">Force an encounter</option>
            <option value="pick">Pick the result</option>
          </select>
        </label>
        <label v-if="mode === 'pick' && chosen" class="g-field">
          <span>Result</span>
          <select v-model.number="entry" data-testid="check-entry">
            <option v-for="(e, i) in chosen.entries" :key="i" :value="i">{{ e.label || e.kind }}</option>
          </select>
        </label>
        <GButton variant="primary" :disabled="!chosen" data-testid="run-check" @click="emit('send', { kind: 'encounter_check', tableId, mode, ...(mode === 'pick' ? { entry } : {}) })">
          Check now
        </GButton>
      </div>
      <div class="row">
        <label class="g-field">
          <span>Or check it at</span>
          <select v-model="due" data-testid="check-due">
            <option value="next_rest">the next rest</option>
            <option value="next_travel">the next travel leg</option>
          </select>
        </label>
        <GButton :disabled="!chosen" data-testid="schedule-check" @click="emit('send', { kind: 'schedule_check', tableId, due })">Schedule</GButton>
      </div>
    </template>
    <p v-if="checks.length === 0" class="hint">No checks yet this session.</p>
    <ol class="g-list">
      <li v-for="c in recent" :key="c.id" :data-testid="`check-${c.id}`">{{ checkLine(c) }}</li>
    </ol>
  </section>
</template>

<style scoped>
.checks {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.grow {
  flex: 1 1 160px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
</style>
