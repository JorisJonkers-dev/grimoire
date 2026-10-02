<script setup lang="ts">
import { reactive } from 'vue'
import type { LiveToken, VisibilityQuality } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ token: LiveToken }>()
const emit = defineEmits<{ set: [visibility: { qualities: VisibilityQuality[]; seenThrough: VisibilityQuality[]; disguise?: string }] }>()
const all: { quality: VisibilityQuality; name: string }[] = [
  { quality: 'hidden', name: 'Hidden' },
  { quality: 'invisible', name: 'Invisible' },
  { quality: 'disguised', name: 'Disguised' },
  { quality: 'illusory', name: 'Illusory' },
  { quality: 'ethereal', name: 'Ethereal' },
  { quality: 'darkness', name: 'In darkness' },
  { quality: 'heavy', name: 'Heavily obscured' },
  { quality: 'secret', name: 'Secret' },
]
const has = (q: VisibilityQuality) => props.token.qualities?.find((x) => x.quality === q)
const form = reactive({
  on: Object.fromEntries(all.map((a) => [a.quality, !!has(a.quality)])) as Record<VisibilityQuality, boolean>,
  through: Object.fromEntries(all.map((a) => [a.quality, !!has(a.quality)?.seenThrough])) as Record<VisibilityQuality, boolean>,
  disguise: props.token.disguise ?? '',
})
function save() {
  const qualities = all.map((a) => a.quality).filter((q) => form.on[q])
  emit('set', {
    qualities,
    seenThrough: qualities.filter((q) => form.through[q]),
    ...(form.on.disguised && form.disguise.trim() ? { disguise: form.disguise.trim() } : {}),
  })
}
</script>

<template>
  <section class="visibility" :aria-label="`Visibility of ${token.label}`" data-testid="visibility-panel">
    <h2>Visibility of {{ token.label }}</h2>
    <ul class="g-list">
      <li v-for="a in all" :key="a.quality" class="row">
        <label class="check"><input v-model="form.on[a.quality]" type="checkbox" :data-testid="`quality-${a.quality}`" /><span>{{ a.name }}</span></label>
        <label v-if="form.on[a.quality]" class="check">
          <input v-model="form.through[a.quality]" type="checkbox" :data-testid="`seen-through-${a.quality}`" /><span>party saw through it</span>
        </label>
      </li>
    </ul>
    <label v-if="form.on.disguised" class="g-field">
      <span>Shows as</span><input v-model="form.disguise" maxlength="40" placeholder="Old woman" data-testid="disguise" />
    </label>
    <GButton data-testid="save-visibility" @click="save()">Save visibility</GButton>
  </section>
</template>

<style scoped>
.visibility {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 16px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
</style>
