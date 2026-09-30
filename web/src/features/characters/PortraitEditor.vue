<script setup lang="ts">
import { ref } from 'vue'
import { type Ids, pictureProblem, uploadPicture } from './pictures'

const props = defineProps<{ ids: Ids }>()
const emit = defineEmits<{ changed: [] }>()
const problem = ref('')
const busy = ref(false)

async function pick(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  const why = pictureProblem(file)
  problem.value = why ?? ''
  if (why) return
  busy.value = true
  try {
    await uploadPicture('portrait', props.ids, file)
    emit('changed')
  } catch {
    problem.value = 'The portrait could not be saved. Try again shortly.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="portrait-editor" data-testid="portrait-editor">
    <label class="upload">
      <span>Upload a portrait</span>
      <input type="file" accept="image/png,image/jpeg,image/webp" :disabled="busy" data-testid="portrait-file" @change="pick" />
    </label>
    <label class="upload">
      <span>Take a photo</span>
      <input type="file" accept="image/png,image/jpeg,image/webp" capture="user" :disabled="busy" @change="pick" />
    </label>
    <p v-if="problem" role="alert" class="g-alert" data-testid="portrait-problem">{{ problem }}</p>
  </div>
</template>

<style scoped>
.portrait-editor {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.upload {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  padding: 0 16px;
  border: 1px solid var(--color-bronze);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  cursor: pointer;
}
.upload input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
}
.upload:focus-within {
  outline: 2px solid var(--color-gold-high);
}
.g-alert {
  flex-basis: 100%;
}
</style>
