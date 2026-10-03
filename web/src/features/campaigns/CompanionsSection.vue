<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { createCompanionMutation, deleteCompanionMutation, listCompanionsOptions, updateCompanionMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Companion, Member } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'

// The allies who travel with the party: a creature under a name of its own, run by a Player or by the
// DM, with or without a share of the XP. Every Member sees them; the DM keeps them.
const props = defineProps<{ campaignId: string; dm: boolean; members: Member[] }>()
const client = useQueryClient()
const path = computed(() => ({ path: { campaignId: props.campaignId } }))
const list = useQuery(computed(() => ({ ...listCompanionsOptions(path.value), retry: false })))
const companions = computed(() => list.data.value ?? [])
const create = useMutation(createCompanionMutation())
const update = useMutation(updateCompanionMutation())
const remove = useMutation(deleteCompanionMutation())

const editing = ref('')
const name = ref('')
const kind = ref<Companion['kind']>('companion')
const creature = ref('')
const controller = ref('')
const shares = ref(false)
const notes = ref('')
const problem = ref('')
const ready = computed(() => name.value.trim() !== '' && creature.value.trim() !== '')

function clear() {
  editing.value = ''
  name.value = ''
  kind.value = 'companion'
  creature.value = ''
  controller.value = ''
  shares.value = false
  notes.value = ''
}
function edit(c: Companion) {
  editing.value = c.id
  name.value = c.name
  kind.value = c.kind
  creature.value = c.monsterSlug
  controller.value = c.controllerId ?? ''
  shares.value = c.sharesXp
  notes.value = c.notes
}
const done = {
  onSuccess: () => { problem.value = ''; clear(); void client.invalidateQueries() },
  onError: (e: unknown) => {
    const p = e as { status?: number; detail?: string } | null
    problem.value = p?.status === 422 && p.detail ? p.detail : 'That could not be saved. Try again shortly.'
  },
}
function save() {
  if (!ready.value) return
  const body = {
    name: name.value.trim(), kind: kind.value, monsterSlug: creature.value.trim(), ...(controller.value ? { controllerId: controller.value } : {}),
    sharesXp: shares.value, notes: notes.value,
  }
  if (editing.value) update.mutate({ path: { campaignId: props.campaignId, companionId: editing.value }, body }, done)
  else create.mutate({ ...path.value, body }, done)
}
const letGo = (c: Companion) => { remove.mutate({ path: { campaignId: props.campaignId, companionId: c.id } }, { ...done, onError: () => { problem.value = 'That could not be saved. Try again shortly.' } }) }
function runBy(c: Companion): string {
  const who = props.members.find((m) => m.id === c.controllerId)
  return who ? (who.isMe ? 'Run by you' : `Run by ${who.displayName}`) : 'Run by the DM'
}
</script>

<template>
  <section v-if="dm || companions.length" class="g-card stack" data-testid="companions">
    <h2>Companions</h2>
    <ul v-if="companions.length" class="g-list">
      <li v-for="c in companions" :key="c.id" class="row" :data-testid="`companion-${c.name}`">
        <span>
          <strong>{{ c.name }}</strong>
          <span class="g-tag">{{ c.kind === 'hireling' ? 'Hireling' : 'Companion' }}</span>
          <span class="dim">
            {{ c.monsterSlug }} · {{ runBy(c) }}<template v-if="c.sharesXp"> · takes a share of the XP</template><template v-if="c.hp !== undefined"> · {{ c.hp }} hit points</template>
          </span>
          <span v-if="c.notes" class="notes">{{ c.notes }}</span>
        </span>
        <span v-if="dm" class="actions">
          <GButton :aria-label="`Change ${c.name}`" :data-testid="`companion-edit-${c.name}`" @click="edit(c)">Change</GButton>
          <GButton variant="danger" :aria-label="`Let ${c.name} go`" :data-testid="`companion-delete-${c.name}`" @click="letGo(c)">Let go</GButton>
        </span>
      </li>
    </ul>
    <p v-else class="dim">No allies travel with the party yet.</p>
    <p v-if="problem" role="alert" class="g-alert" data-testid="companion-problem">{{ problem }}</p>
    <form v-if="dm" class="form" data-testid="companion-form" @submit.prevent="save">
      <h3>{{ editing ? 'Change this ally' : 'Add an ally' }}</h3>
      <GField v-model="name" label="Name" :maxlength="40" data-testid="companion-name" />
      <GField v-model="creature" label="Creature" hint="Its slug in the Compendium or your Library, such as wolf." :maxlength="120" data-testid="companion-creature" />
      <label class="g-field">
        <span>Kind</span>
        <select v-model="kind" data-testid="companion-kind">
          <option value="companion">Companion</option>
          <option value="hireling">Hireling</option>
        </select>
      </label>
      <label class="g-field">
        <span>Run by</span>
        <select v-model="controller" data-testid="companion-controller">
          <option value="">The DM</option>
          <option v-for="m in members" :key="m.id" :value="m.id">{{ m.displayName }}</option>
        </select>
      </label>
      <label class="check"><input v-model="shares" type="checkbox" data-testid="companion-shares" /><span>Takes a share of the XP</span></label>
      <GField v-model="notes" label="Your notes" multiline :maxlength="2000" data-testid="companion-notes" />
      <span class="actions">
        <GButton type="submit" variant="primary" :disabled="!ready || create.isPending.value || update.isPending.value" data-testid="companion-save">{{ editing ? 'Save' : 'Add' }}</GButton>
        <GButton v-if="editing" data-testid="companion-cancel" @click="clear">Cancel</GButton>
      </span>
    </form>
  </section>
</template>

<style scoped>
.stack,
.form {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h3 {
  margin: 0;
  font-size: 16px;
}
.row,
.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.row {
  justify-content: space-between;
}
.check {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.dim,
.notes {
  color: var(--color-text-2);
}
.notes {
  display: block;
}
</style>
