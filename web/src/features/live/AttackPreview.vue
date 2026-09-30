<script setup lang="ts">
import type { LiveAttackPreview } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

defineProps<{ preview: LiveAttackPreview; target: string }>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()
</script>

<template>
  <section class="g-card preview" :aria-label="`${preview.name} against ${target}`" data-testid="attack-preview">
    <h2>{{ preview.name }} against {{ target }}</h2>
    <p class="odds">
      <strong data-testid="hit-chance">{{ preview.hitChance }}% to hit</strong>
      · <span data-testid="damage-range">{{ preview.damageMin }}–{{ preview.damageMax }} damage</span>
      <span class="crit"> (critical up to {{ preview.critMax }})</span>
    </p>
    <ul class="reasons" aria-label="Why">
      <li v-for="r in preview.reasons" :key="r">{{ r }}</li>
    </ul>
    <div class="row">
      <GButton variant="primary" data-testid="confirm-attack" @click="emit('confirm')">Attack</GButton>
      <GButton data-testid="cancel-attack" @click="emit('cancel')">Cancel</GButton>
    </div>
  </section>
</template>

<style scoped>
.preview {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.odds {
  margin: 0;
}
.crit,
.reasons {
  color: var(--color-text-2);
}
.reasons {
  margin: 0;
  padding-left: 18px;
  font-size: 14px;
}
.row {
  display: flex;
  gap: 8px;
}
</style>
