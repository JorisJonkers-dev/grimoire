<script setup lang="ts">
import { computed, ref } from 'vue'
import type { LiveCheckpoint } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'

// The DM keeps named Checkpoints and rewinds to one, or to the start of a round, which every round of a
// fight leaves by itself. A rewind takes back everything since, so it asks before it goes.
const props = withDefaults(defineProps<{ checkpoints: LiveCheckpoint[]; noUndo: boolean; split?: boolean }>(), { split: false })
const emit = defineEmits<{ send: [command: { kind: 'checkpoint'; name: string } | { kind: 'rewind'; checkpointId: string }] }>()
const name = ref('')
const asking = ref<LiveCheckpoint | null>(null)
const newestFirst = computed(() => [...props.checkpoints].reverse())

function keep() {
  const called = name.value.trim()
  if (!called) return
  emit('send', { kind: 'checkpoint', name: called })
  name.value = ''
}
function rewind() {
  if (asking.value) emit('send', { kind: 'rewind', checkpointId: asking.value.id })
  asking.value = null
}
</script>

<template>
  <section class="g-card panel" aria-label="Checkpoints" data-testid="checkpoints">
    <h2>Checkpoints</h2>
    <p v-if="noUndo" data-testid="no-undo">This Campaign is played without undo: nothing is taken back.</p>
    <p v-else-if="split" data-testid="checkpoints-split">Bring the party back together to keep Checkpoints again: one group's Session is not the whole of it.</p>
    <template v-else>
      <form class="keep" data-testid="checkpoint-form" @submit.prevent="keep">
        <GField v-model="name" label="Name this point" :maxlength="60" data-testid="checkpoint-name" />
        <GButton type="submit" :disabled="!name.trim()" data-testid="checkpoint-keep">Keep a checkpoint</GButton>
      </form>
      <div v-if="asking" class="sure" role="alertdialog" aria-label="Rewind the Session" data-testid="rewind-confirm">
        <p>Rewind to {{ asking.name }}? Everything done since is taken back.</p>
        <span class="actions">
          <GButton variant="danger" data-testid="rewind-yes" @click="rewind">Rewind</GButton>
          <GButton data-testid="rewind-no" @click="asking = null">Keep playing</GButton>
        </span>
      </div>
      <ol v-if="checkpoints.length" class="g-list">
        <li v-for="c in newestFirst" :key="c.id" class="row" :data-testid="`checkpoint-${c.id}`">
          <span>
            <strong>{{ c.name }}</strong>
            <span v-if="c.kind === 'round'" class="dim"> Start of the round</span>
          </span>
          <GButton :aria-label="`Rewind to ${c.name}`" :data-testid="`rewind-${c.id}`" @click="asking = c">Rewind here</GButton>
        </li>
      </ol>
      <p v-else class="dim" data-testid="checkpoints-none">None yet. Keep one before something risky; every round of a fight leaves one by itself.</p>
      <p class="dim" data-testid="checkpoints-scope">
        A rewind puts the board, the fight, the fog and spent resources back. Items, coins and rests already finished stay as they are.
      </p>
    </template>
  </section>
</template>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h2,
p {
  margin: 0;
}
.keep,
.row,
.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 8px;
}
.row {
  align-items: center;
  justify-content: space-between;
}
.sure {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--color-enemy);
  border-radius: var(--radius-md);
}
.dim {
  color: var(--color-text-2);
}
</style>
