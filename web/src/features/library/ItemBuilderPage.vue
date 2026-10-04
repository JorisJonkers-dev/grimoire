<script setup lang="ts">
import BuilderCrumbs from './BuilderCrumbs.vue'
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getItemBuildOptions, previewItemMutation, saveItemBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { ItemProperty } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'
import { freshRow, itemKinds, masteries, rarities, rowFields, weaponProperties } from './itemRows'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const item = useQuery(computed(() => ({ ...getItemBuildOptions(path.value), retry: false })))
const preview = useMutation(previewItemMutation())
const save = useMutation(saveItemBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: item.data, key: computed(() => getItemBuildOptions(path.value).queryKey), preview, save, fallback: 'Item',
})
// copy takes plain data out of a reactive value.
const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) as T
const adding = ref('skill_boost')
const words = (s: string) => s.replaceAll('_', ' ')

function addRow(type: string, extra: Partial<ItemProperty> = {}) {
  const row = freshRow[type]
  if (row && design.value) design.value.properties.push({ ...copy(row), ...extra })
}
function removeRow(i: number) {
  design.value?.properties.splice(i, 1)
}
function setAttunement(on: boolean) {
  if (design.value) design.value.attunement = on ? {} : undefined
}
function setCharges(on: boolean) {
  if (design.value) design.value.charges = on ? { max: 3, on: 'dawn', dice: 1, faces: 4 } : undefined
}
function setWeapon(on: boolean) {
  if (design.value) design.value.weapon = on ? { properties: [] } : undefined
}
function toggleProperty(p: string) {
  const w = design.value?.weapon
  if (w) w.properties = w.properties.includes(p) ? w.properties.filter((x) => x !== p) : [...w.properties, p]
}
function field(row: ItemProperty, key: keyof ItemProperty): string | number | boolean {
  return row[key] ?? ''
}
function setField(row: ItemProperty, key: keyof ItemProperty, value: string | number | boolean) {
  ;(row as Record<string, unknown>)[key] = value
}
</script>

<template>
  <main class="g-page builder">
    <BuilderCrumbs :entry-id="String(route.params.entryId)" here="Item builder" />
    <p v-if="item.isError.value" role="alert" class="g-alert" data-testid="item-error">This entry cannot be opened in the item builder.</p>
    <p v-else-if="!design">Opening the item…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Item builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="item-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="item-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="item-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="item-form" @submit.prevent="runSave">
          <fieldset class="grid">
            <legend>The item</legend>
            <label class="g-field"><span>Kind</span>
              <select v-model="design.kind" data-testid="item-kind"><option v-for="k in itemKinds" :key="k" :value="k">{{ k }}</option></select>
            </label>
            <label class="g-field"><span>Base item (slug)</span><input v-model="design.base" maxlength="80" data-testid="item-base" /></label>
            <label class="g-field"><span>Rarity</span>
              <select v-model="design.rarity" data-testid="item-rarity"><option v-for="r in rarities" :key="r" :value="r">{{ words(r) }}</option></select>
            </label>
            <label class="g-field"><span>Enchantment</span>
              <select v-model.number="design.enchantment" data-testid="item-enchantment"><option v-for="n in [0, 1, 2, 3]" :key="n" :value="n">+{{ n }}</option></select>
            </label>
            <label class="g-field"><span>Weight (lb)</span><input v-model.number="design.weightLb" type="number" min="0" step="0.5" data-testid="item-weight" /></label>
            <label class="g-field"><span>Value (gp)</span><input v-model.number="design.valueGp" type="number" min="0" data-testid="item-value" /></label>
          </fieldset>
          <fieldset class="grid">
            <legend>Attunement and charges</legend>
            <label class="check">
              <input :checked="Boolean(design.attunement)" type="checkbox" data-testid="item-attunes" @change="setAttunement(($event.target as HTMLInputElement).checked)" />
              <span>Requires attunement</span>
            </label>
            <template v-if="design.attunement">
              <label class="g-field"><span>By a</span>
                <select v-model="design.attunement.kind" data-testid="attune-kind">
                  <option value="">anyone</option>
                  <option v-for="k in ['class', 'species', 'background', 'alignment']" :key="k" :value="k">{{ k }}</option>
                </select>
              </label>
              <label v-if="design.attunement.kind" class="g-field"><span>Which</span><input v-model="design.attunement.value" maxlength="40" data-testid="attune-value" /></label>
            </template>
            <label class="check">
              <input :checked="Boolean(design.charges)" type="checkbox" data-testid="item-charged" @change="setCharges(($event.target as HTMLInputElement).checked)" />
              <span>Holds charges</span>
            </label>
            <template v-if="design.charges">
              <label class="g-field"><span>Charges</span><input v-model.number="design.charges.max" type="number" min="1" data-testid="charges-max" /></label>
              <label class="g-field"><span>Regains</span>
                <select v-model="design.charges.on" data-testid="charges-on"><option value="dawn">at dawn</option><option value="long_rest">on a long rest</option><option value="short_rest">on a short rest</option></select>
              </label>
              <label class="g-field"><span>Dice</span><input v-model.number="design.charges.dice" type="number" min="0" data-testid="charges-dice" /></label>
              <label class="g-field"><span>Faces</span><input v-model.number="design.charges.faces" type="number" min="0" data-testid="charges-faces" /></label>
              <label class="g-field"><span>Bonus</span><input v-model.number="design.charges.bonus" type="number" min="0" data-testid="charges-bonus" /></label>
            </template>
          </fieldset>
          <fieldset v-if="design.kind === 'weapon'" class="grid">
            <legend>Weapon</legend>
            <label class="check">
              <input :checked="Boolean(design.weapon)" type="checkbox" data-testid="item-weapon" @change="setWeapon(($event.target as HTMLInputElement).checked)" />
              <span>Weapon properties and mastery</span>
            </label>
            <template v-if="design.weapon">
              <label v-for="p in weaponProperties" :key="p" class="check">
                <input type="checkbox" :checked="design.weapon.properties.includes(p)" :data-testid="`weapon-${p}`" @change="toggleProperty(p)" />
                <span>{{ p }}</span>
              </label>
              <label class="g-field"><span>Mastery</span>
                <select v-model="design.weapon.mastery" data-testid="weapon-mastery"><option v-for="m in masteries" :key="m" :value="m">{{ m || 'none' }}</option></select>
              </label>
              <label v-if="design.weapon.mastery === 'custom'" class="g-field wide"><span>Custom mastery</span><input v-model="design.weapon.custom" maxlength="300" data-testid="weapon-custom" /></label>
            </template>
          </fieldset>

          <h2>Item Properties</h2>
          <div class="row">
            <GButton data-testid="quick-boost" @click="addRow('skill_boost')">+ Skill Boost</GButton>
            <GButton data-testid="quick-cantrip" @click="addRow('cantrip')">+ Cantrip</GButton>
            <label class="g-field"><span>Add a property</span>
              <select v-model="adding" data-testid="add-property"><option v-for="t in Object.keys(rowFields)" :key="t" :value="t">{{ words(t) }}</option></select>
            </label>
            <GButton data-testid="add-row" @click="addRow(adding)">Add</GButton>
          </div>
          <ol class="parts" data-testid="item-rows">
            <li v-for="(row, i) in design.properties" :key="i" class="part" :data-testid="`row-${String(i)}`">
              <span class="kind">{{ words(row.type) }}</span>
              <template v-for="f in rowFields[row.type] ?? []" :key="f.key">
                <label v-if="f.kind === 'check'" class="check">
                  <input type="checkbox" :checked="Boolean(row[f.key])" :data-testid="`row-${String(i)}-${f.key}`" @change="setField(row, f.key, ($event.target as HTMLInputElement).checked)" />
                  <span>{{ f.label }}</span>
                </label>
                <label v-else-if="f.kind === 'select'" class="g-field"><span>{{ f.label }}</span>
                  <select :value="field(row, f.key)" :data-testid="`row-${String(i)}-${f.key}`" @change="setField(row, f.key, ($event.target as HTMLSelectElement).value)">
                    <option v-for="o in f.options" :key="o" :value="o">{{ words(o) || 'any' }}</option>
                  </select>
                </label>
                <label v-else class="g-field"><span>{{ f.label }}</span>
                  <input
                    :value="field(row, f.key)"
                    :type="f.kind === 'number' ? 'number' : 'text'"
                    :data-testid="`row-${String(i)}-${f.key}`"
                    @input="setField(row, f.key, f.kind === 'number' ? Number(($event.target as HTMLInputElement).value) : ($event.target as HTMLInputElement).value)"
                  />
                </label>
              </template>
              <label class="check"><input v-model="row.hidden" type="checkbox" :data-testid="`row-${String(i)}-hidden`" /><span>Hidden until known</span></label>
              <button type="button" class="drop" :aria-label="`Remove property ${String(i + 1)}`" :data-testid="`row-${String(i)}-remove`" @click="removeRow(i)">×</button>
            </li>
          </ol>
          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="item-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="item-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="Item card" data-testid="item-card">
          <h2>{{ shown?.card[0] ?? name }}</h2>
          <p v-if="shown?.card[1]" class="hint">{{ shown.card[1] }}</p>
          <p v-for="(line, i) in shown?.card.slice(2) ?? []" :key="i" class="line">{{ line }}</p>
          <section v-if="shown" class="price" data-testid="price-check">
            <h3>Price Check</h3>
            <p>
              <span :class="['g-tag', shown.price.fits ? 'fits' : 'off']" data-testid="price-fits">{{ shown.price.fits ? 'Fits' : 'Check it' }}</span>
              Properties point to {{ words(shown.price.suggested) }}; guide price {{ shown.price.priceGp }} gp.
            </p>
            <p v-for="n in shown.price.notes" :key="n" class="hint">{{ n }}</p>
          </section>
        </aside>
      </div>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.builder {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h1,
h2,
h3 {
  margin: 0;
  font-family: var(--font-display);
}
h3 {
  font-size: 15px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.line {
  margin: 0;
}
.layout {
  display: grid;
  gap: 14px;
}
@media (min-width: 960px) {
  .layout {
    grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr);
    align-items: start;
  }
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 8px 12px;
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.wide {
  grid-column: 1 / -1;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.parts {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}
.part {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px 12px;
  padding: 10px 0;
  border-top: 1px solid var(--color-rule);
}
.kind {
  min-width: 90px;
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.drop {
  min-width: 44px;
  min-height: 44px;
  margin-left: auto;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text);
  cursor: pointer;
}
input,
select {
  min-height: 44px;
  min-width: 0;
}
.price {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px solid var(--color-rule);
}
</style>
