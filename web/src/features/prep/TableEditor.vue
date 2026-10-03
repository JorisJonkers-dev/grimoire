<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import type { EncounterEntry, EncounterPool, EncounterTable, EncounterTableInput, EncounterVisibility, Faction, Location } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { formatMonsters, parseMonsters } from './monsters'

type Row = { weight: number; kind: EncounterEntry['kind']; label: string; poolId: string; monsters: string; factionId: string }

const props = defineProps<{ table?: EncounterTable; pools: EncounterPool[]; locations: Location[]; factions: Faction[] }>()
const emit = defineEmits<{ save: [input: EncounterTableInput]; cancel: [] }>()
const form = reactive({ name: props.table?.name ?? '', chancePct: props.table?.chancePct ?? 20, regionId: props.table?.regionId ?? '' })
const visibility = ref<EncounterVisibility>(props.table?.visibility ?? 'secret')
const rows = ref<Row[]>(
  props.table?.entries.map((e) => ({ weight: e.weight, kind: e.kind, label: e.label, poolId: e.poolId ?? '', monsters: formatMonsters(e.monsters), factionId: e.factionId ?? '' })) ?? [
    { weight: 1, kind: 'nothing', label: '', poolId: '', monsters: '', factionId: '' },
  ],
)
const unreadable = computed(() => rows.value.findIndex((r) => r.kind === 'encounter' && parseMonsters(r.monsters) === null))
function entry(r: Row): EncounterEntry {
  // An entry that is a Faction's own weighs by how that Faction regards the party.
  const base = { weight: r.weight, kind: r.kind, label: r.label.trim(), ...(r.factionId ? { factionId: r.factionId } : {}) }
  if (r.kind === 'pool') return { ...base, poolId: r.poolId }
  if (r.kind === 'encounter') return { ...base, monsters: parseMonsters(r.monsters) ?? [] }
  return base
}
function save() {
  emit('save', {
    name: form.name.trim(), chancePct: form.chancePct, visibility: visibility.value, entries: rows.value.map(entry),
    ...(form.regionId ? { regionId: form.regionId } : {}),
  })
}
</script>

<template>
  <form class="g-card editor" :aria-label="table ? `Edit ${table.name}` : 'New table'" data-testid="table-editor" @submit.prevent="save">
    <label class="g-field"><span>Name</span><input v-model="form.name" maxlength="80" data-testid="table-name" /></label>
    <div class="row">
      <label class="g-field">
        <span>Region</span>
        <select v-model="form.regionId" data-testid="table-region">
          <option value="">Everywhere</option>
          <option v-for="l in locations" :key="l.id" :value="l.id">{{ l.name }} ({{ l.mapName }})</option>
        </select>
      </label>
      <label class="g-field"><span>Chance (%)</span><input v-model.number="form.chancePct" type="number" min="0" max="100" data-testid="table-chance" /></label>
      <label class="g-field">
        <span>Checks are</span>
        <select v-model="visibility" data-testid="table-visibility">
          <option value="secret">Secret: players see only the outcome</option>
          <option value="open">Open: the roll shows on the table</option>
        </select>
      </label>
    </div>
    <fieldset class="entries">
      <legend>Entries</legend>
      <div v-for="(r, i) in rows" :key="i" class="row">
        <label class="g-field"><span>Weight</span><input v-model.number="r.weight" type="number" min="1" max="100" :data-testid="`entry-weight-${String(i)}`" /></label>
        <label class="g-field">
          <span>Kind</span>
          <select v-model="r.kind" :data-testid="`entry-kind-${String(i)}`">
            <option value="encounter">Prepared encounter</option>
            <option value="pool">Draw from a pool</option>
            <option value="nothing">Nothing</option>
          </select>
        </label>
        <label class="g-field grow"><span>Label</span><input v-model="r.label" maxlength="80" :data-testid="`entry-label-${String(i)}`" /></label>
        <label v-if="r.kind === 'pool'" class="g-field">
          <span>Pool</span>
          <select v-model="r.poolId" :data-testid="`entry-pool-${String(i)}`">
            <option v-for="p in pools" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </label>
        <label class="g-field">
          <span>Faction's own</span>
          <select v-model="r.factionId" :data-testid="`entry-faction-${String(i)}`">
            <option value="">No Faction</option>
            <option v-for="f in factions" :key="f.id" :value="f.id">{{ f.name }}</option>
          </select>
        </label>
        <label v-if="r.kind === 'encounter'" class="g-field grow">
          <span>Creatures (goblin x3, ogre)</span>
          <input v-model="r.monsters" maxlength="400" :data-testid="`entry-monsters-${String(i)}`" />
        </label>
        <GButton :aria-label="`Remove entry ${String(i + 1)}`" :disabled="rows.length === 1" @click="rows.splice(i, 1)">Remove</GButton>
      </div>
      <GButton data-testid="add-entry" @click="rows.push({ weight: 1, kind: 'encounter', label: '', poolId: '', monsters: '', factionId: '' })">Add an entry</GButton>
    </fieldset>
    <p v-if="unreadable >= 0" role="alert" class="g-alert" data-testid="monsters-unreadable">Entry {{ unreadable + 1 }}: write creatures as "goblin x3, ogre".</p>
    <div class="row">
      <GButton type="submit" variant="primary" :disabled="form.name.trim() === '' || unreadable >= 0" data-testid="save-table">Save table</GButton>
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
