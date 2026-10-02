<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  getInventoryOptions,
  getInventoryQueryKey,
  moveItemMutation,
  takeFromStashMutation,
  useItemMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { EquipmentSlot, InventoryView, ItemCard } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const client = useQueryClient()
const ids = computed(() => ({ campaignId: String(route.params.id), characterId: String(route.params.characterId) }))
const inventory = useQuery(computed(() => ({ ...getInventoryOptions({ path: ids.value }), retry: false })))
const inv = computed(() => inventory.data.value)
const move = useMutation(moveItemMutation())
const take = useMutation(takeFromStashMutation())
const use = useMutation(useItemMutation())
const problem = computed(() => move.error.value ?? take.error.value ?? use.error.value)
const status = ref('')

const slotNames: Record<EquipmentSlot, string> = {
  head: 'Head', cloak: 'Cloak', neck: 'Amulet', armor: 'Body', hands: 'Hands', ring_1: 'Ring', ring_2: 'Ring', feet: 'Feet',
  main_hand: 'Main hand', off_hand: 'Off hand', ranged_main: 'Ranged', ammunition: 'Ammunition', instrument: 'Instrument',
}
const filter = ref('')
const sortBy = ref<'name' | 'weight' | 'category'>('name')
const bag = computed(() => {
  const words = filter.value.trim().toLowerCase()
  const items = (inv.value?.bag ?? []).filter((c) => !words || `${c.name} ${c.category}`.toLowerCase().includes(words))
  const key = { name: (c: ItemCard) => c.name, weight: (c: ItemCard) => -c.weightLb * c.quantity, category: (c: ItemCard) => `${c.category} ${c.name}` }[sortBy.value]
  return [...items].sort((a, b) => {
    const x = key(a)
    const y = key(b)
    return typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y))
  })
})
const pct = computed(() => (inv.value ? Math.min(100, Math.round((inv.value.weightLb / Math.max(inv.value.capacityLb, 1)) * 100)) : 0))
const loadText = { none: 'Carrying comfortably', encumbered: 'Over capacity: speed drops to 5 feet', immobile: 'Too heavy to move' }
const coins = (list: InventoryView['coins']) => list.map((c) => `${String(c.count)} ${c.coin}`).join(', ') || 'No coins'

const ref_ = (c: ItemCard) => (c.instanceId ? { instanceId: c.instanceId } : { slug: c.slug })
function done(view: InventoryView, message = '') {
  client.setQueryData(getInventoryQueryKey({ path: ids.value }), view)
  void client.invalidateQueries({ predicate: (q) => q.queryKey[0] !== getInventoryQueryKey({ path: ids.value })[0] })
  status.value = message
}
function send(c: ItemCard, to: 'bag' | 'slot' | 'character' | 'stash', extra: { slot?: EquipmentSlot; characterId?: string } = {}) {
  status.value = ''
  move.mutate({ path: ids.value, body: { ...ref_(c), to, count: to === 'slot' || to === 'bag' ? 1 : c.quantity, ...extra } }, { onSuccess: (v) => { done(v) } })
}
function takeOut(c: ItemCard) {
  status.value = ''
  take.mutate({ path: ids.value, body: { ...ref_(c), count: c.quantity } }, { onSuccess: (v) => { done(v) } })
}
function act(c: ItemCard, what: 'drink' | 'throw') {
  status.value = ''
  use.mutate(
    { path: ids.value, body: { ...ref_(c), use: what } },
    {
      onSuccess: (r) => {
        done(r.inventory, what === 'drink' ? (r.healed ? `${c.name}: ${String(r.healed)} hit points back.` : `You drink ${c.name}.`) : `You throw away ${c.name}.`)
      },
    },
  )
}

// Drag and drop: what is being dragged, and from where.
const dragging = ref<{ card: ItemCard; from: 'bag' | 'slot' | 'stash' } | null>(null)
function dragStart(ev: DragEvent, card: ItemCard, from: 'bag' | 'slot' | 'stash') {
  dragging.value = { card, from }
  ev.dataTransfer?.setData('text/plain', card.name)
}
function dropOn(target: 'bag' | 'stash' | { slot: EquipmentSlot } | { character: string }) {
  const d = dragging.value
  dragging.value = null
  if (!d) return
  if (d.from === 'stash') {
    if (target === 'bag') takeOut(d.card)
    return
  }
  if (target === 'bag') send(d.card, 'bag')
  else if (target === 'stash') send(d.card, 'stash')
  else if ('slot' in target) {
    if (d.card.fits.includes(target.slot)) send(d.card, 'slot', { slot: target.slot })
    else status.value = `${d.card.name} does not go there.`
  } else send(d.card, 'character', { characterId: target.character })
}
const giveTo = ref<Record<string, string>>({})
const key = (c: ItemCard) => c.instanceId ?? c.slug
</script>

<template>
  <main class="g-page inventory">
    <RouterLink :to="{ name: 'character', params: { id: ids.campaignId, characterId: ids.characterId } }" class="back">← Sheet</RouterLink>
    <h1>Inventory<template v-if="inv"> · {{ inv.name }}</template></h1>
    <p v-if="inventory.isError.value" role="alert" class="g-alert" data-testid="inventory-error">This Inventory could not be opened.</p>
    <p v-else-if="!inv">Unpacking…</p>
    <template v-else>
      <section class="load" aria-label="Carrying" data-testid="carrying">
        <span>{{ inv.weightLb.toFixed(1) }} / {{ inv.capacityLb }} lb · {{ loadText[inv.load] }}</span>
        <span class="bar" role="img" :aria-label="`${String(pct)} percent of carrying capacity`"><span :class="inv.load" :style="{ width: `${String(pct)}%` }" /></span>
        <span class="hint">{{ coins(inv.coins) }}</span>
      </section>
      <p v-if="status" role="status" class="g-tag" data-testid="inventory-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="inventory-problem">{{ problem.detail ?? 'That did not work.' }}</p>

      <div class="layout">
        <section class="figure" aria-label="Equipment" data-testid="equipment">
          <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the item buttons are the accessible path -->
          <div
            v-for="s in inv.slots"
            :key="s.slot"
            :class="['slot', `at-${s.slot}`, { filled: s.item }]"
            :data-testid="`slot-${s.slot}`"
            @dragover.prevent
            @drop.prevent="dropOn({ slot: s.slot })"
          >
            <span class="label">{{ slotNames[s.slot] }}</span>
            <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the item buttons are the accessible path -->
            <span
              v-if="s.item"
              class="worn"
              draggable="true"
              :data-testid="`worn-${s.slot}`"
              @dragstart="dragStart($event, s.item, 'slot')"
            >{{ s.item.customName ?? s.item.name }}</span>
            <button v-if="s.item" type="button" class="remove" :aria-label="`Take off ${s.item.name}`" :data-testid="`unequip-${s.slot}`" @click="send(s.item, 'bag')">×</button>
          </div>
        </section>

        <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the item buttons are the accessible path -->
        <section class="g-card bag" aria-label="Bag" data-testid="bag" @dragover.prevent @drop.prevent="dropOn('bag')">
          <h2>Bag</h2>
          <div class="tools">
            <label class="pick"><span class="label">Find</span><input v-model="filter" type="search" data-testid="bag-filter" /></label>
            <label class="pick">
              <span class="label">Sort by</span>
              <select v-model="sortBy" data-testid="bag-sort">
                <option value="name">Name</option>
                <option value="weight">Weight</option>
                <option value="category">Kind</option>
              </select>
            </label>
          </div>
          <p v-if="bag.length === 0" class="hint">Nothing here.</p>
          <ul class="cards">
            <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the item buttons are the accessible path -->
            <li v-for="c in bag" :key="key(c)" class="card" draggable="true" :data-testid="`item-${c.slug}`" @dragstart="dragStart($event, c, 'bag')">
              <span class="name">{{ c.customName ?? c.name }}<small v-if="c.quantity > 1"> ×{{ c.quantity }}</small></span>
              <small class="meta">{{ c.category }} · {{ (c.weightLb * c.quantity).toFixed(1) }} lb</small>
              <span class="actions">
                <select v-if="c.fits.length" :aria-label="`Equip ${c.name}`" :data-testid="`equip-${c.slug}`" @change="send(c, 'slot', { slot: ($event.target as HTMLSelectElement).value as EquipmentSlot })">
                  <option value="">Equip…</option>
                  <option v-for="f in c.fits" :key="f" :value="f">{{ slotNames[f] }}</option>
                </select>
                <GButton v-if="c.category === 'potion'" :data-testid="`drink-${c.slug}`" @click="act(c, 'drink')">Drink</GButton>
                <select v-if="inv.party.length" v-model="giveTo[key(c)]" :aria-label="`Give ${c.name} to`" :data-testid="`give-to-${c.slug}`">
                  <option value="">Give to…</option>
                  <option v-for="p in inv.party" :key="p.characterId" :value="p.characterId">{{ p.name }}</option>
                </select>
                <GButton v-if="giveTo[key(c)]" :data-testid="`give-${c.slug}`" @click="send(c, 'character', { characterId: giveTo[key(c)] })">Give</GButton>
                <GButton :data-testid="`stash-${c.slug}`" @click="send(c, 'stash')">Stash</GButton>
                <GButton variant="danger" :data-testid="`throw-${c.slug}`" @click="act(c, 'throw')">Throw</GButton>
              </span>
            </li>
          </ul>
          <div v-if="inv.party.length" class="party" aria-label="Give by dropping on a Character">
            <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the item buttons are the accessible path -->
            <span
              v-for="p in inv.party"
              :key="p.characterId"
              class="g-tag ally"
              :data-testid="`ally-${p.name}`"
              @dragover.prevent
              @drop.prevent="dropOn({ character: p.characterId })"
            >{{ p.name }}</span>
          </div>
        </section>

        <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the item buttons are the accessible path -->
        <section class="g-card stash" aria-label="Party Stash" data-testid="stash" @dragover.prevent @drop.prevent="dropOn('stash')">
          <h2>Party Stash</h2>
          <p class="hint">{{ coins(inv.stashCoins) }}</p>
          <p v-if="inv.stash.length === 0" class="hint">The stash is empty.</p>
          <ul class="cards">
            <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the item buttons are the accessible path -->
            <li v-for="c in inv.stash" :key="key(c)" class="card" draggable="true" :data-testid="`stash-item-${c.slug}`" @dragstart="dragStart($event, c, 'stash')">
              <span class="name">{{ c.customName ?? c.name }}<small v-if="c.quantity > 1"> ×{{ c.quantity }}</small></span>
              <GButton :data-testid="`take-${c.slug}`" @click="takeOut(c)">Take</GButton>
            </li>
          </ul>
        </section>
      </div>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.inventory {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h1,
h2 {
  margin: 0;
  font-family: var(--font-display);
}
h2 {
  font-size: 17px;
}
.hint,
.label,
.meta {
  color: var(--color-text-2);
}
.label,
.meta {
  font-size: 12px;
}
.load {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.bar {
  height: 8px;
  border-radius: var(--radius-pill);
  background: var(--color-raised);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  background: var(--color-success);
}
.bar span.encumbered,
.bar span.immobile {
  background: var(--color-enemy-soft);
}
.layout {
  display: grid;
  gap: 14px;
}
@media (min-width: 960px) {
  .layout {
    grid-template-columns: 360px 1fr;
  }
  .stash {
    grid-column: 1 / -1;
  }
}
.figure {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  grid-template-areas:
    'main head ranged'
    'off cloak neck'
    'hands armor ring1'
    'ammo feet ring2'
    'instrument instrument instrument';
  gap: 6px;
}
.slot {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 64px;
  padding: 6px 8px;
  border: 1px dashed var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.slot.filled {
  border-style: solid;
  border-color: var(--color-bronze);
}
.at-head { grid-area: head; }
.at-cloak { grid-area: cloak; }
.at-neck { grid-area: neck; }
.at-armor { grid-area: armor; }
.at-hands { grid-area: hands; }
.at-ring_1 { grid-area: ring1; }
.at-ring_2 { grid-area: ring2; }
.at-feet { grid-area: feet; }
.at-main_hand { grid-area: main; }
.at-off_hand { grid-area: off; }
.at-ranged_main { grid-area: ranged; }
.at-ammunition { grid-area: ammo; }
.at-instrument { grid-area: instrument; }
.worn {
  cursor: grab;
  overflow-wrap: anywhere;
}
.remove {
  position: absolute;
  top: 2px;
  right: 2px;
  min-width: 32px;
  min-height: 32px;
  border: 0;
  background: none;
  color: var(--color-text-2);
  font-size: 18px;
  cursor: pointer;
}
.tools {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 8px 0;
}
.pick {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
input,
select {
  min-height: 44px;
  padding: 0 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font: inherit;
}
.cards {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.card {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 10px;
  padding: 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  cursor: grab;
}
.name {
  flex: 1 1 140px;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.party {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}
.ally {
  min-height: 36px;
  padding: 8px 12px;
}
</style>
