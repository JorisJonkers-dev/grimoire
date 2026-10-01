<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { Coin, LootEntry, LootTable, LootTableInput } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

type Row = { weight: number; kind: LootEntry['kind']; itemSlug: string; coin: Coin; amount: string; tableId: string }

const props = defineProps<{ table?: LootTable; tables: LootTable[] }>()
const emit = defineEmits<{ save: [input: LootTableInput]; cancel: [] }>()
const form = reactive({ name: props.table?.name ?? '', rolls: props.table?.rolls ?? 1 })
const rows = ref<Row[]>(
  props.table?.entries.map((e) => ({ weight: e.weight, kind: e.kind, itemSlug: e.itemSlug ?? '', coin: e.coin ?? 'gp', amount: e.amount ?? '1', tableId: e.tableId ?? '' })) ?? [
    { weight: 1, kind: 'currency', itemSlug: '', coin: 'gp', amount: '2d6', tableId: '' },
  ],
)
const others = () => props.tables.filter((t) => t.id !== props.table?.id)
function entry(r: Row): LootEntry {
  const base = { weight: r.weight, kind: r.kind }
  if (r.kind === 'item') return { ...base, itemSlug: r.itemSlug.trim(), amount: r.amount.trim() }
  if (r.kind === 'currency') return { ...base, coin: r.coin, amount: r.amount.trim() }
  if (r.kind === 'table') return { ...base, tableId: r.tableId }
  return base
}
function save() {
  emit('save', { name: form.name.trim(), rolls: form.rolls, entries: rows.value.map(entry) })
}
</script>

<template>
  <form class="g-card editor" :aria-label="table ? `Edit ${table.name}` : 'New loot table'" data-testid="loot-editor" @submit.prevent="save">
    <div class="row">
      <label class="g-field grow"><span>Name</span><input v-model="form.name" maxlength="80" data-testid="loot-name" /></label>
      <label class="g-field"><span>Rolls</span><input v-model.number="form.rolls" type="number" min="1" max="10" data-testid="loot-rolls" /></label>
    </div>
    <fieldset class="entries">
      <legend>Entries</legend>
      <div v-for="(r, i) in rows" :key="i" class="row">
        <label class="g-field"><span>Weight</span><input v-model.number="r.weight" type="number" min="1" max="100" :data-testid="`loot-weight-${String(i)}`" /></label>
        <label class="g-field">
          <span>Kind</span>
          <select v-model="r.kind" :data-testid="`loot-kind-${String(i)}`">
            <option value="item">Item</option>
            <option value="currency">Coins</option>
            <option value="table">Roll another table</option>
            <option value="nothing">Nothing</option>
          </select>
        </label>
        <label v-if="r.kind === 'item'" class="g-field grow"><span>Item</span><input v-model="r.itemSlug" maxlength="80" placeholder="rope" :data-testid="`loot-item-${String(i)}`" /></label>
        <label v-if="r.kind === 'currency'" class="g-field">
          <span>Coin</span>
          <select v-model="r.coin" :data-testid="`loot-coin-${String(i)}`">
            <option v-for="c in ['cp', 'sp', 'ep', 'gp', 'pp'] as const" :key="c" :value="c">{{ c }}</option>
          </select>
        </label>
        <label v-if="r.kind === 'item' || r.kind === 'currency'" class="g-field">
          <span>Amount (3, 2d6, 4d6x10)</span>
          <input v-model="r.amount" maxlength="20" :data-testid="`loot-amount-${String(i)}`" />
        </label>
        <label v-if="r.kind === 'table'" class="g-field">
          <span>Table</span>
          <select v-model="r.tableId" :data-testid="`loot-table-${String(i)}`">
            <option v-for="t in others()" :key="t.id" :value="t.id">{{ t.name }}</option>
          </select>
        </label>
        <GButton :aria-label="`Remove loot entry ${String(i + 1)}`" :disabled="rows.length === 1" @click="rows.splice(i, 1)">Remove</GButton>
      </div>
      <GButton data-testid="add-loot-entry" @click="rows.push({ weight: 1, kind: 'item', itemSlug: '', coin: 'gp', amount: '1', tableId: '' })">Add an entry</GButton>
    </fieldset>
    <div class="row">
      <GButton type="submit" variant="primary" :disabled="form.name.trim() === ''" data-testid="save-loot">Save loot table</GButton>
      <GButton @click="emit('cancel')">Cancel</GButton>
    </div>
  </form>
</template>

<style scoped>
.editor,
.entries {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.entries {
  margin: 0;
  padding: 0;
  border: 0;
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
