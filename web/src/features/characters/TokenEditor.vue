<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { GButton, TokenBadge } from '@/shared/ui'
import { cropToBlob, type Ids, pictureProblem, uploadPicture, showInitials } from './pictures'

const props = defineProps<{ name: string; ids: Ids; portraitUrl?: string; tokenUrl?: string }>()
const emit = defineEmits<{ changed: [] }>()

type Mode = 'initials' | 'crop' | 'icon'
const mode = ref<Mode>(props.tokenUrl ? 'icon' : 'initials')
const zoom = ref(1)
const x = ref(0)
const y = ref(0)
const busy = ref(false)
const problem = ref('')
const preview = ref('')
const image = ref<HTMLImageElement | null>(null)
let pending: Blob | null = null

const shown = computed(() => (mode.value === 'initials' ? '' : preview.value || props.tokenUrl || ''))
const sizes = [
  { label: 'Initiative', px: 32 },
  { label: 'Map', px: 48 },
  { label: 'Table', px: 96 },
]

function setPreview(blob: Blob | null) {
  if (preview.value) URL.revokeObjectURL(preview.value)
  pending = blob
  preview.value = blob ? URL.createObjectURL(blob) : ''
}
onBeforeUnmount(() => {
  setPreview(null)
})

async function recrop() {
  if (mode.value !== 'crop' || !image.value) return
  setPreview(await cropToBlob(image.value, { zoom: zoom.value, x: x.value, y: y.value }))
}
watch([zoom, x, y, mode], () => void recrop())

async function run(action: () => Promise<void>) {
  busy.value = true
  problem.value = ''
  try {
    await action()
    emit('changed')
  } catch {
    problem.value = 'The token could not be saved. Try again shortly.'
  } finally {
    busy.value = false
  }
}

function pickIcon(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  const why = pictureProblem(file)
  problem.value = why ?? ''
  if (!why) setPreview(file)
}

function save() {
  if (mode.value === 'initials') {
    void run(() => showInitials(props.ids))
    return
  }
  const blob = pending
  if (blob) void run(() => uploadPicture('token', props.ids, blob))
}
</script>

<template>
  <section class="g-card token-editor" data-testid="token-editor">
    <h2>Token</h2>
    <fieldset>
      <legend>Show on the token</legend>
      <label class="choice"><input v-model="mode" type="radio" value="initials" /><span>Initials</span></label>
      <label class="choice">
        <input v-model="mode" type="radio" value="crop" :disabled="!portraitUrl" /><span>Crop from the portrait</span>
      </label>
      <label class="choice"><input v-model="mode" type="radio" value="icon" /><span>A separate icon</span></label>
    </fieldset>
    <div v-if="mode === 'crop' && portraitUrl" class="crop">
      <img ref="image" :src="portraitUrl" alt="" crossorigin="use-credentials" class="source" @load="recrop()" />
      <label class="g-field"><span>Zoom</span><input v-model.number="zoom" type="range" min="1" max="4" step="0.1" data-testid="crop-zoom" /></label>
      <label class="g-field"><span>Left and right</span><input v-model.number="x" type="range" min="-1" max="1" step="0.05" /></label>
      <label class="g-field"><span>Up and down</span><input v-model.number="y" type="range" min="-1" max="1" step="0.05" /></label>
    </div>
    <label v-if="mode === 'icon'" class="g-field">
      <span>Icon (PNG, JPEG or WebP, up to 10 MB)</span>
      <input type="file" accept="image/png,image/jpeg,image/webp" data-testid="icon-file" @change="pickIcon" />
    </label>
    <div class="previews" data-testid="token-previews">
      <figure v-for="s in sizes" :key="s.label">
        <TokenBadge :name="name" allegiance="party" :icon-url="shown" :size="s.px" />
        <figcaption>{{ s.label }}</figcaption>
      </figure>
    </div>
    <p v-if="problem" role="alert" class="g-alert" data-testid="token-problem">{{ problem }}</p>
    <GButton variant="primary" :disabled="busy || (mode !== 'initials' && !preview)" data-testid="save-token" @click="save()">Save token</GButton>
  </section>
</template>

<style scoped>
.token-editor {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h2 {
  margin: 0;
}
fieldset {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 6px;
}
.choice {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
}
.source {
  max-width: 100%;
  max-height: 200px;
  object-fit: contain;
}
.crop {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.previews {
  display: flex;
  align-items: flex-end;
  gap: 16px;
}
figure {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  margin: 0;
  font-size: 12px;
  color: var(--color-text-2);
}
</style>
