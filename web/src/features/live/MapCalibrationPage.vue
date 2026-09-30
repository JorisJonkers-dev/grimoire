<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, reactive, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getMapOptions, updateMapMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LiveMap, MapEdit } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import MapBoard from './MapBoard.vue'

const route = useRoute()
const path = { path: { campaignId: String(route.params.id), mapId: String(route.params.mapId) } }
const map = useQuery({ ...getMapOptions(path), retry: false })
const form = reactive<MapEdit>({ name: '', hexSizePx: 40, originX: 0, originY: 0, ambient: 'bright' })
watch(
  () => map.data.value,
  (m) => {
    if (m) Object.assign(form, { name: m.name, hexSizePx: m.hexSizePx, originX: m.originX, originY: m.originY, ambient: m.ambient })
  },
  { immediate: true },
)
const preview = computed<LiveMap | null>(() =>
  map.data.value
    ? { ...map.data.value, hexSizePx: form.hexSizePx, originX: form.originX, originY: form.originY, imageVersion: 0 }
    : null,
)
const save = useMutation(updateMapMutation())
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'maps', params: { id: path.path.campaignId } }" class="back">← Maps</RouterLink>
    <p v-if="map.isError.value" role="alert" class="g-alert" data-testid="map-missing">That map is not available.</p>
    <template v-else-if="preview">
      <h1>{{ form.name }}</h1>
      <p class="hint">Line the grid up with the picture: set the hex size and move the origin onto the centre of one hex.</p>
      <form class="g-card calibrate" data-testid="map-calibrate" @submit.prevent="save.mutate({ ...path, body: { ...form } })">
        <label class="g-field"><span>Name</span><input v-model="form.name" maxlength="80" /></label>
        <label class="g-field"><span>Hex size (px, centre to corner)</span><input v-model.number="form.hexSizePx" type="number" min="8" max="400" step="any" data-testid="hex-size" /></label>
        <label class="g-field"><span>Origin x (px)</span><input v-model.number="form.originX" type="number" step="any" data-testid="origin-x" /></label>
        <label class="g-field"><span>Origin y (px)</span><input v-model.number="form.originY" type="number" step="any" /></label>
        <label class="g-field">
          <span>Ambient light</span>
          <select v-model="form.ambient">
            <option value="bright">Bright</option>
            <option value="dim">Dim</option>
            <option value="dark">Dark</option>
          </select>
        </label>
        <GButton type="submit" variant="primary">Save calibration</GButton>
        <p v-if="save.isSuccess.value" role="status" data-testid="calibration-saved">Saved.</p>
        <p v-if="save.isError.value" role="alert" class="g-alert">The calibration could not be saved.</p>
      </form>
      <MapBoard :map="preview" :view="{ tokens: [], fog: false, visible: [], remembered: [] }" dm :title="`${form.name} with its hex grid`" />
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.calibrate {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  align-items: end;
}
</style>
