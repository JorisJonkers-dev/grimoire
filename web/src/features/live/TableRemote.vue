<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import type { LiveTable, LocalMap, TableCamera, TableScene } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ table?: LiveTable; maps: LocalMap[] }>()
const emit = defineEmits<{
  camera: [camera: TableCamera, zoomPct: number]
  scene: [scene: { scene: TableScene; title?: string; body?: string; mapId?: string }]
  blackout: [on: boolean]
}>()
const zoom = ref(props.table?.zoomPct ?? 100)
watch(
  () => props.table?.zoomPct,
  (z) => {
    if (z) zoom.value = z
  },
)
const form = reactive({ title: '', body: '', mapId: '' })
const scene = ref<TableScene>('title')
const cameras: { value: TableCamera; label: string }[] = [
  { value: 'follow_turn', label: 'Follow the turn' },
  { value: 'show_party', label: 'Show the party' },
  { value: 'free', label: 'Free (tap the map with "Point the table camera")' },
]
function show() {
  emit('scene', {
    scene: scene.value,
    ...(scene.value === 'title' || scene.value === 'handout' ? { title: form.title, body: form.body } : {}),
    ...(scene.value === 'world' ? { mapId: form.mapId } : {}),
  })
}
</script>

<template>
  <section class="remote" aria-label="Table display" data-testid="table-remote">
    <h2>Table display</h2>
    <fieldset class="row">
      <legend>Camera</legend>
      <label v-for="c in cameras" :key="c.value" class="check">
        <input
          type="radio"
          name="table-camera"
          :checked="table?.camera === c.value"
          :data-testid="`camera-${c.value}`"
          @change="emit('camera', c.value, zoom)"
        />
        <span>{{ c.label }}</span>
      </label>
      <label class="g-field">
        <span>Zoom {{ zoom }}%</span>
        <input v-model.number="zoom" type="range" min="50" max="300" step="10" data-testid="camera-zoom" @change="emit('camera', table?.camera ?? 'follow_turn', zoom)" />
      </label>
    </fieldset>
    <form class="row" @submit.prevent="show()">
      <label class="g-field">
        <span>Scene</span>
        <select v-model="scene" data-testid="scene">
          <option value="local">Local map</option>
          <option value="world">World map</option>
          <option value="handout">Handout</option>
          <option value="title">Title card</option>
        </select>
      </label>
      <template v-if="scene === 'title' || scene === 'handout'">
        <label class="g-field"><span>Title</span><input v-model="form.title" maxlength="80" data-testid="scene-title" /></label>
        <label class="g-field grow"><span>Text</span><input v-model="form.body" maxlength="1000" data-testid="scene-body" /></label>
      </template>
      <label v-if="scene === 'world'" class="g-field">
        <span>Map</span>
        <select v-model="form.mapId" data-testid="scene-map">
          <option v-for="m in maps" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select>
      </label>
      <GButton type="submit" data-testid="show-scene">Show on the table</GButton>
    </form>
    <GButton :variant="table?.blackout ? 'primary' : 'danger'" data-testid="blackout-toggle" @click="emit('blackout', !table?.blackout)">
      {{ table?.blackout ? 'Lights back on' : 'Blackout' }}
    </GButton>
  </section>
</template>

<style scoped>
.remote {
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
  margin: 0;
  padding: 0;
  border: 0;
}
.grow {
  flex: 1 1 200px;
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
</style>
