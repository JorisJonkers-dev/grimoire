<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import type { Ability, LiveToken } from '@/infrastructure/api/types.gen'
import { GButton, StatusIcon } from '@/shared/ui'
import { effectLabel, knownEffects } from './conditions'

const props = defineProps<{ token: LiveToken; tokens: LiveToken[] }>()
const emit = defineEmits<{
  apply: [effect: { effect: string; sourceId?: string; rounds?: number; saveAbility?: Ability; saveDc?: number; effectMode?: string; monsterSlug?: string; tempHp?: number }]
  end: [effectId: string]
}>()
const known = knownEffects
const abilities: Ability[] = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma']
const form = reactive({ effect: '', source: '', rounds: 0, dc: 0, mode: '', creature: '', temp: 0 })
const saveWith = ref<Ability | ''>('')
// ends names the concentration the chosen source would lose by putting on a concentration Effect.
const ends = computed(() => {
  if (!form.source || !knownEffects.find((k) => k.slug === form.effect.trim())?.concentration) return ''
  const held = props.tokens.flatMap((t) => (t.effects ?? []).filter((e) => e.concentration && e.sourceId === form.source))
  const who = props.tokens.find((t) => t.id === form.source)?.label ?? 'The source'
  return held.length ? `${who} stops concentrating on ${held.map((e) => e.name).join(', ')}.` : ''
})
function apply() {
  const slug = form.effect.trim()
  const ability = saveWith.value
  if (!slug) return
  emit('apply', {
    effect: slug,
    ...(form.source ? { sourceId: form.source } : {}),
    ...(form.rounds ? { rounds: form.rounds } : {}),
    ...(ability && form.dc ? { saveAbility: ability, saveDc: form.dc } : {}),
    ...(form.mode.trim() ? { effectMode: form.mode.trim() } : {}),
    ...(form.creature.trim() ? { monsterSlug: form.creature.trim() } : {}),
    ...(form.temp > 0 ? { tempHp: form.temp } : {}),
  })
  form.creature = ''
  form.temp = 0
  form.effect = ''
  form.mode = ''
}
</script>

<template>
  <section class="effects" :aria-label="`Effects on ${props.token.label}`" data-testid="effects-panel">
    <h2>Effects on {{ token.label }}</h2>
    <ul v-if="token.effects?.length" class="g-list">
      <li v-for="e in token.effects" :key="e.id" class="effect">
        <span class="what">
          <StatusIcon :slug="e.slug" :label="effectLabel(e)" />
          {{ effectLabel(e, true) }}
        </span>
        <GButton variant="danger" :data-testid="`end-effect-${e.slug}`" @click="emit('end', e.id)">End</GButton>
      </li>
    </ul>
    <form class="row" @submit.prevent="apply()">
      <label class="g-field grow">
        <span>Effect</span>
        <input v-model="form.effect" list="known-effects" placeholder="bless" maxlength="80" data-testid="effect-name" />
        <datalist id="known-effects">
          <option v-for="k in known" :key="k.slug" :value="k.slug">{{ k.name }}</option>
        </datalist>
      </label>
      <label class="g-field">
        <span>From</span>
        <select v-model="form.source" data-testid="effect-source">
          <option value="">Nobody</option>
          <option v-for="t in tokens" :key="t.id" :value="t.id">{{ t.label }}</option>
        </select>
      </label>
      <label class="g-field"><span>Mode</span><input v-model="form.mode" maxlength="80" placeholder="Enlarge" data-testid="effect-mode" /></label>
      <label class="g-field"><span>Becomes</span><input v-model="form.creature" maxlength="80" placeholder="wolf" data-testid="effect-creature" /></label>
      <label class="g-field"><span>Temp HP</span><input v-model.number="form.temp" type="number" min="0" max="999" data-testid="effect-temp" /></label>
      <label class="g-field"><span>Rounds</span><input v-model.number="form.rounds" type="number" min="0" max="100" data-testid="effect-rounds" /></label>
      <label class="g-field">
        <span>Ends on a save</span>
        <select v-model="saveWith" data-testid="effect-save">
          <option value="">No save</option>
          <option v-for="a in abilities" :key="a" :value="a">{{ a }}</option>
        </select>
      </label>
      <label class="g-field"><span>DC</span><input v-model.number="form.dc" type="number" min="0" max="40" data-testid="effect-dc" /></label>
      <p v-if="ends" role="alert" class="warn" data-testid="concentration-warning">{{ ends }}</p>
      <GButton type="submit" data-testid="apply-effect">Apply</GButton>
    </form>
  </section>
</template>

<style scoped>
.warn {
  margin: 0;
  color: var(--color-enemy-soft);
}
.what {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.effects {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 16px;
}
.effect {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.grow {
  flex: 1 1 140px;
}
.row input[type='number'] {
  width: 5em;
}
</style>
