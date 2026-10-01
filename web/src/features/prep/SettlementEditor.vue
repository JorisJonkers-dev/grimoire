<script setup lang="ts">
import { reactive } from 'vue'
import type { Location, Settlement, SettlementInput } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ settlement?: Settlement; locations: Location[] }>()
const emit = defineEmits<{ save: [input: SettlementInput]; cancel: [] }>()
const form = reactive({
  name: props.settlement?.name ?? '',
  size: props.settlement?.size ?? 'village',
  wealth: props.settlement?.wealth ?? 'modest',
  locationId: props.settlement?.locationId ?? '',
})
function save() {
  const input: SettlementInput = { name: form.name.trim(), size: form.size, wealth: form.wealth }
  if (form.locationId) input.locationId = form.locationId
  emit('save', input)
}
</script>

<template>
  <form class="g-card editor" :aria-label="settlement ? `Edit ${settlement.name}` : 'New settlement'" data-testid="settlement-editor" @submit.prevent="save">
    <label class="g-field grow"><span>Name</span><input v-model="form.name" maxlength="80" data-testid="settlement-name" /></label>
    <label class="g-field">
      <span>Size</span>
      <select v-model="form.size" data-testid="settlement-size">
        <option v-for="s in ['hamlet', 'village', 'town', 'city'] as const" :key="s" :value="s">{{ s }}</option>
      </select>
    </label>
    <label class="g-field">
      <span>Wealth</span>
      <select v-model="form.wealth" data-testid="settlement-wealth">
        <option v-for="w in ['poor', 'modest', 'comfortable', 'wealthy'] as const" :key="w" :value="w">{{ w }}</option>
      </select>
    </label>
    <label class="g-field">
      <span>World map location</span>
      <select v-model="form.locationId" data-testid="settlement-location">
        <option value="">None</option>
        <option v-for="l in locations" :key="l.id" :value="l.id">{{ l.name }} ({{ l.mapName }})</option>
      </select>
    </label>
    <div class="row">
      <GButton type="submit" variant="primary" :disabled="form.name.trim() === ''" data-testid="save-settlement">Save settlement</GButton>
      <GButton @click="emit('cancel')">Cancel</GButton>
    </div>
  </form>
</template>

<style scoped>
.editor {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.row {
  display: flex;
  gap: 8px;
}
.grow {
  flex: 1 1 160px;
}
</style>
