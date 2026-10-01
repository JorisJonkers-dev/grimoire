<script setup lang="ts">
import type { LiveAreaPreview } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

defineProps<{ preview: LiveAreaPreview; names: Record<string, string> }>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()
</script>

<template>
  <section class="g-card area" :aria-label="`${preview.name} preview`" data-testid="area-preview">
    <h2>{{ preview.name }} · DC {{ preview.dc }}</h2>
    <p v-if="preview.allies > 0" role="alert" class="warn" data-testid="ally-warning">
      This catches {{ preview.allies }} {{ preview.allies === 1 ? 'ally' : 'allies' }}.
    </p>
    <p v-if="preview.ends?.length" role="alert" class="warn" data-testid="concentration-warning">
      Casting this ends your concentration on {{ preview.ends.join(', ') }}.
    </p>
    <p v-if="preview.targets.length === 0" data-testid="area-empty">Nobody stands in the area.</p>
    <ul v-else class="g-list" aria-label="Caught in the area">
      <li v-for="t in preview.targets" :key="t.tokenId">{{ names[t.tokenId] ?? 'Someone' }}{{ t.ally ? ' (ally)' : '' }}</li>
    </ul>
    <div class="row">
      <GButton variant="primary" data-testid="confirm-area" @click="emit('confirm')">Cast</GButton>
      <GButton data-testid="cancel-area" @click="emit('cancel')">Cancel</GButton>
    </div>
  </section>
</template>

<style scoped>
.area {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.warn {
  margin: 0;
  font-weight: 600;
  color: var(--color-enemy-soft);
}
.row {
  display: flex;
  gap: 8px;
}
</style>
