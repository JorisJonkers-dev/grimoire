<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { EncounterDifficulty, EncounterPool, EncounterPoolInput, EncounterPoolMember } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ pool?: EncounterPool }>()
const emit = defineEmits<{ save: [input: EncounterPoolInput]; cancel: [] }>()
const form = reactive({ name: props.pool?.name ?? '', levelMin: props.pool?.levelMin ?? 1, levelMax: props.pool?.levelMax ?? 4 })
const difficulty = ref<EncounterDifficulty>(props.pool?.difficulty ?? 'moderate')
const members = ref<EncounterPoolMember[]>(props.pool?.members.map((m) => ({ ...m })) ?? [{ monsterSlug: '', weight: 1, min: 0, max: 4 }])
function save() {
  emit('save', { name: form.name.trim(), levelMin: form.levelMin, levelMax: form.levelMax, difficulty: difficulty.value, members: members.value.map((m) => ({ ...m, monsterSlug: m.monsterSlug.trim() })) })
}
</script>

<template>
  <form class="g-card editor" :aria-label="pool ? `Edit ${pool.name}` : 'New pool'" data-testid="pool-editor" @submit.prevent="save">
    <label class="g-field"><span>Name</span><input v-model="form.name" maxlength="80" data-testid="pool-name" /></label>
    <div class="row">
      <label class="g-field"><span>From level</span><input v-model.number="form.levelMin" type="number" min="1" max="20" data-testid="pool-level-min" /></label>
      <label class="g-field"><span>To level</span><input v-model.number="form.levelMax" type="number" min="1" max="20" data-testid="pool-level-max" /></label>
      <label class="g-field">
        <span>Difficulty</span>
        <select v-model="difficulty" data-testid="pool-difficulty">
          <option value="low">Low</option>
          <option value="moderate">Moderate</option>
          <option value="high">High</option>
        </select>
      </label>
    </div>
    <fieldset class="members">
      <legend>Creatures</legend>
      <div v-for="(m, i) in members" :key="i" class="row">
        <label class="g-field grow"><span>Monster</span><input v-model="m.monsterSlug" maxlength="80" placeholder="goblin" :data-testid="`member-slug-${String(i)}`" /></label>
        <label class="g-field"><span>Weight</span><input v-model.number="m.weight" type="number" min="1" max="100" :data-testid="`member-weight-${String(i)}`" /></label>
        <label class="g-field"><span>At least</span><input v-model.number="m.min" type="number" min="0" max="20" :data-testid="`member-min-${String(i)}`" /></label>
        <label class="g-field"><span>At most</span><input v-model.number="m.max" type="number" min="1" max="20" :data-testid="`member-max-${String(i)}`" /></label>
        <GButton :aria-label="`Remove creature ${String(i + 1)}`" :disabled="members.length === 1" @click="members.splice(i, 1)">Remove</GButton>
      </div>
      <GButton data-testid="add-member" @click="members.push({ monsterSlug: '', weight: 1, min: 0, max: 4 })">Add a creature</GButton>
    </fieldset>
    <div class="row">
      <GButton type="submit" variant="primary" :disabled="form.name.trim() === ''" data-testid="save-pool">Save pool</GButton>
      <GButton @click="emit('cancel')">Cancel</GButton>
    </div>
  </form>
</template>

<style scoped>
.editor,
.members {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.members {
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
