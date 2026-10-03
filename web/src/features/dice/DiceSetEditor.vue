<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { clearDiceSetImageMutation, createDiceSetMutation, editDiceSetMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { DiceSet } from '@/infrastructure/api/types.gen'
import { pictureProblem } from '@/features/characters/pictures'
import { GButton, GField } from '@/shared/ui'
import DiceSheet from './DiceSheet.vue'
import { CENTRED, DIE_TYPES, PATTERNS, everyDie, type DieType } from './sets'
import { uploadSetPicture } from './upload'

// A Dice Set is designed die by die: a pattern and two colours, and the set's picture placed on the
// die's unwrapped faces. A new set is saved before it can carry a picture.
const props = defineProps<{ set?: DiceSet }>()
const emit = defineEmits<{ saved: [id: string]; done: [] }>()
const client = useQueryClient()
const name = ref(props.set?.name ?? '')
const dice = reactive(everyDie(props.set?.design.dice ?? {}))
const die = ref<DieType>('d20')
const look = computed(() => dice[die.value])
const problem = ref('')
const busy = ref(false)

const create = useMutation(createDiceSetMutation())
const edit = useMutation(editDiceSetMutation())
const unpicture = useMutation(clearDiceSetImageMutation())
const failed = () => { problem.value = 'The Dice Set could not be saved. Try again shortly.' }

function save() {
  problem.value = ''
  const body = { name: name.value.trim(), design: { dice: JSON.parse(JSON.stringify(dice)) as typeof dice } }
  const done = { onSuccess: (saved: DiceSet) => { void client.invalidateQueries(); emit('saved', saved.id) }, onError: failed }
  if (props.set) edit.mutate({ path: { diceSetId: props.set.id }, body }, done)
  else create.mutate({ body }, done)
}
function everyDieLikeThis() {
  for (const t of DIE_TYPES) dice[t] = { ...look.value, ...(look.value.image ? { image: { ...look.value.image } } : {}) }
}
function place(on: boolean) {
  if (on) look.value.image = { ...CENTRED }
  else delete look.value.image
}
async function pick(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file || !props.set) return
  const why = pictureProblem(file)
  problem.value = why ?? ''
  if (why) return
  busy.value = true
  try {
    await uploadSetPicture(props.set.id, file)
    await client.invalidateQueries()
  } catch {
    problem.value = 'The picture could not be saved. Try again shortly.'
  } finally {
    busy.value = false
  }
}
function removePicture() {
  if (props.set) unpicture.mutate({ path: { diceSetId: props.set.id } }, { onSuccess: () => void client.invalidateQueries(), onError: failed })
}
</script>

<template>
  <form class="g-card editor" data-testid="dice-set-editor" @submit.prevent="save">
    <h2>{{ set ? `Edit ${set.name}` : 'A new Dice Set' }}</h2>
    <GField v-model="name" label="Name" :maxlength="60" required data-testid="set-name" />
    <div class="dice" role="group" aria-label="Which die to dress">
      <button v-for="t in DIE_TYPES" :key="t" type="button" class="die" :aria-pressed="die === t" :data-testid="`die-${t}`" @click="die = t">{{ t }}</button>
    </div>
    <div class="design">
      <DiceSheet :type="die" :look="look" :image-url="set?.imageUrl" />
      <div class="fields">
        <label class="g-field">
          <span>Pattern</span>
          <select v-model="look.pattern" data-testid="look-pattern">
            <option v-for="p in PATTERNS" :key="p" :value="p">{{ p }}</option>
          </select>
        </label>
        <label class="g-field colour">
          <span>Colour</span>
          <input v-model="look.body" type="color" data-testid="look-body" />
        </label>
        <label class="g-field colour">
          <span>Numbers</span>
          <input v-model="look.numbers" type="color" data-testid="look-numbers" />
        </label>
        <GButton type="button" data-testid="look-all" @click="everyDieLikeThis">Use this look on every die</GButton>
      </div>
    </div>

    <fieldset class="picture">
      <legend>Picture</legend>
      <p v-if="!set" class="hint" data-testid="picture-later">Save the set first; then you can put a picture on it.</p>
      <template v-else>
        <label class="upload">
          <span>{{ set.hasImage ? 'Upload another picture' : 'Upload a picture' }}</span>
          <input type="file" accept="image/png,image/jpeg,image/webp" :disabled="busy" data-testid="set-picture-file" @change="pick" />
        </label>
        <template v-if="set.hasImage">
          <GButton type="button" data-testid="set-picture-remove" @click="removePicture">Take the picture off</GButton>
          <label class="check">
            <input type="checkbox" :checked="Boolean(look.image)" data-testid="picture-on" @change="place(($event.target as HTMLInputElement).checked)" />
            <span>Put the picture on the {{ die }}</span>
          </label>
          <div v-if="look.image" class="sliders">
            <label><span>Across</span><input v-model.number="look.image.x" type="range" min="0" max="1" step="0.01" data-testid="picture-x" /></label>
            <label><span>Down</span><input v-model.number="look.image.y" type="range" min="0" max="1" step="0.01" data-testid="picture-y" /></label>
            <label><span>Size</span><input v-model.number="look.image.scale" type="range" min="0.1" max="4" step="0.05" data-testid="picture-scale" /></label>
            <label><span>Turn</span><input v-model.number="look.image.rotation" type="range" min="-180" max="180" step="1" data-testid="picture-rotation" /></label>
          </div>
        </template>
        <p class="hint">A set with a picture is checked by an Admin before everyone can see it.</p>
      </template>
    </fieldset>

    <p v-if="problem" role="alert" class="g-alert" data-testid="set-problem">{{ problem }}</p>
    <div class="actions">
      <GButton type="submit" variant="primary" :disabled="!name.trim() || create.isPending.value || edit.isPending.value" data-testid="set-save">Save</GButton>
      <GButton type="button" data-testid="set-done" @click="emit('done')">Close</GButton>
    </div>
  </form>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.dice,
.actions,
.design {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.design {
  gap: 16px;
}
.die {
  min-width: 48px;
  min-height: 44px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  color: var(--color-text);
  background: var(--color-raised);
  cursor: pointer;
}
.die[aria-pressed='true'] {
  border-color: var(--color-gold-high);
  background: var(--color-surface);
}
.fields {
  display: flex;
  flex: 1 1 220px;
  flex-direction: column;
  gap: 10px;
}
.colour input {
  width: 100%;
  min-height: 44px;
  padding: 2px;
}
.picture {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0;
  padding: 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
}
.upload {
  position: relative;
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
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
  outline-offset: 2px;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.sliders {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px;
}
.sliders label {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.sliders input {
  min-height: 44px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
</style>
