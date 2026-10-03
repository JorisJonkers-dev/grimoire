<script setup lang="ts">
import { reactive } from 'vue'
import type { Faction, LootTable, Npc, Settlement, Shop, ShopInput } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ shop?: Shop; settlements: Settlement[]; npcs: Npc[]; factions: Faction[]; lootTables: LootTable[] }>()
const emit = defineEmits<{ save: [input: ShopInput]; cancel: [] }>()
const form = reactive({
  settlementId: props.shop?.settlementId ?? props.settlements[0]?.id ?? '',
  name: props.shop?.name ?? '',
  kind: props.shop?.kind ?? 'general',
  ownerId: props.shop?.ownerId ?? '',
  factionId: props.shop?.factionId ?? '',
  markupPct: props.shop?.markupPct ?? 0,
  haggleDc: props.shop?.haggleDc ?? 15,
  hagglePct: props.shop?.hagglePct ?? 10,
  lootTableId: props.shop?.lootTableId ?? '',
  restock: props.shop?.restock ?? ('long_rest'),
  restockDays: props.shop?.restockDays ?? 7,
})
function save() {
  const input: ShopInput = {
    settlementId: form.settlementId, name: form.name.trim(), kind: form.kind.trim(), markupPct: form.markupPct,
    haggleDc: form.haggleDc, hagglePct: form.hagglePct, restock: form.restock,
  }
  if (form.ownerId) input.ownerId = form.ownerId
  if (form.factionId) input.factionId = form.factionId
  if (form.lootTableId) input.lootTableId = form.lootTableId
  if (form.restock === 'days') input.restockDays = form.restockDays
  emit('save', input)
}
</script>

<template>
  <form class="g-card editor" :aria-label="shop ? `Edit ${shop.name}` : 'New shop'" data-testid="shop-editor" @submit.prevent="save">
    <div class="row">
      <label class="g-field grow"><span>Name</span><input v-model="form.name" maxlength="80" data-testid="shop-name" /></label>
      <label class="g-field"><span>Kind</span><input v-model="form.kind" maxlength="40" data-testid="shop-kind" /></label>
      <label class="g-field">
        <span>Settlement</span>
        <select v-model="form.settlementId" data-testid="shop-settlement">
          <option v-for="s in settlements" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
      </label>
      <label class="g-field">
        <span>Owner</span>
        <select v-model="form.ownerId" data-testid="shop-owner">
          <option value="">Nobody in particular</option>
          <option v-for="n in npcs" :key="n.id" :value="n.id">{{ n.name }}</option>
        </select>
      </label>
      <label class="g-field">
        <span>Faction (prices follow its Standing)</span>
        <select v-model="form.factionId" data-testid="shop-faction">
          <option value="">No Faction</option>
          <option v-for="f in factions" :key="f.id" :value="f.id">{{ f.name }}</option>
        </select>
      </label>
    </div>
    <div class="row">
      <label class="g-field"><span>Markup %</span><input v-model.number="form.markupPct" type="number" min="0" max="300" data-testid="shop-markup" /></label>
      <label class="g-field"><span>Haggle DC</span><input v-model.number="form.haggleDc" type="number" min="5" max="30" data-testid="shop-haggle-dc" /></label>
      <label class="g-field"><span>Haggle swing %</span><input v-model.number="form.hagglePct" type="number" min="0" max="50" data-testid="shop-haggle-pct" /></label>
    </div>
    <div class="row">
      <label class="g-field">
        <span>Stock from loot table</span>
        <select v-model="form.lootTableId" data-testid="shop-loot">
          <option value="">Stock by hand</option>
          <option v-for="t in lootTables" :key="t.id" :value="t.id">{{ t.name }}</option>
        </select>
      </label>
      <label class="g-field">
        <span>Restock</span>
        <select v-model="form.restock" data-testid="shop-restock">
          <option value="never">Never</option>
          <option value="long_rest">After a long rest</option>
          <option value="days">Every few days</option>
        </select>
      </label>
      <label v-if="form.restock === 'days'" class="g-field">
        <span>Days</span>
        <input v-model.number="form.restockDays" type="number" min="1" max="365" data-testid="shop-restock-days" />
      </label>
    </div>
    <div class="row">
      <GButton type="submit" variant="primary" :disabled="form.name.trim() === '' || form.settlementId === ''" data-testid="save-shop">Save shop</GButton>
      <GButton @click="emit('cancel')">Cancel</GButton>
    </div>
  </form>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
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
</style>
