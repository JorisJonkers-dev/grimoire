<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import type { LiveContainer, LootTable } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { GButton } from '@/shared/ui'
import { canPut, canTake, instanceLabel, type Dragged, load } from './inventory'

const props = defineProps<{ containers: LiveContainer[]; dm: boolean; me: string; lootTables: LootTable[] }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()
const lootTable = ref('')
// Where each stack goes and how many, keyed by container and item or coin.
const target = reactive<Record<string, string>>({})
const count = reactive<Record<string, number>>({})
const over = ref('')
const order = ['party_stash', 'character', 'bag', 'loot_drop']
const sorted = computed(() => [...props.containers].sort((a, b) => order.indexOf(a.kind) - order.indexOf(b.kind)))
const destinations = (from: LiveContainer) => sorted.value.filter((c) => c.id !== from.id && canPut(c, props.dm, props.me))

function move(d: Dragged, to: string) {
  if (d.coin) emit('send', { kind: 'move_coins', fromId: d.from, toId: to, coin: d.coin, count: d.count })
  else if (d.instanceId) emit('send', { kind: 'move_item', fromId: d.from, toId: to, instanceId: d.instanceId })
  else emit('send', { kind: 'move_item', fromId: d.from, toId: to, itemSlug: d.itemSlug ?? '', count: d.count })
}
function moveStack(c: LiveContainer, key: string, d: Dragged) {
  const to = target[`${c.id}:${key}`] ?? destinations(c)[0]?.id
  if (to) move({ ...d, count: Math.min(count[`${c.id}:${key}`] ?? d.count, d.count) }, to)
}
// Whose claims a member makes: their own Characters, or any for the DM.
const claimants = computed(() => props.containers.filter((c) => c.kind === 'character' && c.characterId && (props.dm || c.ownerId === props.me)))
const claimant = ref('')
const claimFor = computed(() => claimant.value || claimants.value[0]?.characterId || '')
const pileItems = (c: LiveContainer) => [
  ...c.items.map((i) => ({ key: i.slug, name: i.name, ref: { itemSlug: i.slug } })),
  ...c.instances.map((i) => ({ key: i.id, name: instanceLabel(i), ref: { instanceId: i.id } })),
]
function claim(c: LiveContainer, ref: { itemSlug?: string; instanceId?: string }, option: 'need' | 'greed' | 'pass') {
  emit('send', { kind: 'claim_loot', fromId: c.id, characterId: claimFor.value, option, ...ref })
}
function drag(ev: DragEvent, d: Dragged) {
  ev.dataTransfer?.setData('application/json', JSON.stringify(d))
}
function drop(ev: DragEvent, c: LiveContainer) {
  over.value = ''
  const raw = ev.dataTransfer?.getData('application/json')
  if (!raw || !canPut(c, props.dm, props.me)) return
  const d = JSON.parse(raw) as Dragged
  if (d.from !== c.id) move(d, c.id)
}
</script>

<template>
  <section class="g-card inventory" aria-label="Inventory" data-testid="inventory">
    <h2>Inventory</h2>
    <div v-if="dm" class="row">
      <label class="g-field grow">
        <span>Loot table</span>
        <select v-model="lootTable" data-testid="roll-loot-table">
          <option value="" disabled>Choose a loot table</option>
          <option v-for="t in lootTables" :key="t.id" :value="t.id">{{ t.name }}</option>
        </select>
      </label>
      <GButton :disabled="!lootTable" data-testid="roll-loot" @click="emit('send', { kind: 'roll_loot', lootTableId: lootTable })">Roll loot</GButton>
    </div>
    <p class="hint">Drag items between packs, or choose where they go below.</p>
    <div class="containers">
      <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the Move controls are the accessible path -->
      <article
        v-for="c in sorted"
        :key="c.id"
        :class="['g-card', 'container', `container--${c.kind}`, { over: over === c.id }]"
        :aria-label="c.label"
        :data-testid="`container-${c.label}`"
        @dragover.prevent="over = canPut(c, dm, me) ? c.id : ''"
        @dragleave="over = ''"
        @drop.prevent="drop($event, c)"
      >
        <h3>
          {{ c.label }} <span class="weight" data-testid="load">{{ load(c) }}</span>
          <span v-if="c.encumbered" class="g-tag heavy" data-testid="encumbered">Encumbered</span>
        </h3>
        <p v-if="!dm && c.ownerId && c.ownerId !== me" class="hint" data-testid="private">Private.</p>
        <p v-else-if="c.items.length + c.instances.length + c.coins.length === 0" class="hint">Empty.</p>
        <ul class="g-list">
          <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the Move controls are the accessible path -->
          <li
            v-for="i in c.items"
            :key="i.slug"
            class="stack"
            :draggable="canTake(c, dm, me)"
            @dragstart="drag($event, { from: c.id, itemSlug: i.slug, count: i.count })"
          >
            <span>{{ i.name }} ×{{ i.count }} · {{ Math.round(i.weightLb * 10) / 10 }} lb</span>
            <template v-if="canTake(c, dm, me) && destinations(c).length">
              <select v-model="target[`${c.id}:${i.slug}`]" :aria-label="`Where ${i.name} goes`">
                <option v-for="d in destinations(c)" :key="d.id" :value="d.id">{{ d.label }}</option>
              </select>
              <input v-model.number="count[`${c.id}:${i.slug}`]" type="number" min="1" :max="i.count" :placeholder="String(i.count)" :aria-label="`How many ${i.name}`" />
              <GButton :aria-label="`Move ${i.name} from ${c.label}`" @click="moveStack(c, i.slug, { from: c.id, itemSlug: i.slug, count: i.count })">Move</GButton>
            </template>
          </li>
          <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the Move controls are the accessible path -->
          <li
            v-for="i in c.instances"
            :key="i.id"
            class="stack"
            data-testid="instance"
            :draggable="canTake(c, dm, me)"
            @dragstart="drag($event, { from: c.id, instanceId: i.id, count: i.count })"
          >
            <span>{{ instanceLabel(i) }} · {{ Math.round(i.weightLb * 10) / 10 }} lb</span>
            <template v-if="canTake(c, dm, me) && destinations(c).length">
              <select v-model="target[`${c.id}:${i.id}`]" :aria-label="`Where ${i.name} goes`">
                <option v-for="d in destinations(c)" :key="d.id" :value="d.id">{{ d.label }}</option>
              </select>
              <GButton :aria-label="`Move ${i.name} from ${c.label}`" @click="moveStack(c, i.id, { from: c.id, instanceId: i.id, count: i.count })">Move</GButton>
            </template>
          </li>
          <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the Move controls are the accessible path -->
          <li
            v-for="k in c.coins"
            :key="k.coin"
            class="stack"
            :draggable="canTake(c, dm, me)"
            @dragstart="drag($event, { from: c.id, coin: k.coin, count: k.count })"
          >
            <span>{{ k.count }} {{ k.coin }}</span>
            <template v-if="canTake(c, dm, me) && destinations(c).length">
              <select v-model="target[`${c.id}:${k.coin}`]" :aria-label="`Where the ${k.coin} goes`">
                <option v-for="d in destinations(c)" :key="d.id" :value="d.id">{{ d.label }}</option>
              </select>
              <input v-model.number="count[`${c.id}:${k.coin}`]" type="number" min="1" :max="k.count" :placeholder="String(k.count)" :aria-label="`How many ${k.coin}`" />
              <GButton :aria-label="`Move ${k.coin} from ${c.label}`" @click="moveStack(c, k.coin, { from: c.id, coin: k.coin, count: k.count })">Move</GButton>
            </template>
          </li>
        </ul>
        <div v-if="c.kind === 'loot_drop'" class="claims" data-testid="claims">
          <label v-if="claimants.length > 1" class="g-field">
            <span>Claim for</span>
            <select v-model="claimant" data-testid="claim-for">
              <option v-for="k in claimants" :key="k.id" :value="k.characterId">{{ k.label }}</option>
            </select>
          </label>
          <div v-for="it in pileItems(c)" :key="it.key" class="claim">
            <span>{{ it.name }}</span>
            <span v-for="cl in (c.claims ?? []).filter((x) => x.item === it.key)" :key="cl.characterId" class="g-tag" :data-testid="`claim-${cl.name}-${it.key}`">
              {{ cl.name }}: {{ cl.choice }} {{ cl.roll }}
            </span>
            <template v-if="claimFor">
              <GButton :data-testid="`need-${it.key}`" @click="claim(c, it.ref, 'need')">Need</GButton>
              <GButton :data-testid="`greed-${it.key}`" @click="claim(c, it.ref, 'greed')">Greed</GButton>
              <GButton :data-testid="`pass-${it.key}`" @click="claim(c, it.ref, 'pass')">Pass</GButton>
            </template>
          </div>
          <GButton v-if="dm" variant="primary" data-testid="settle-loot" @click="emit('send', { kind: 'settle_loot', fromId: c.id })">Share out</GButton>
          <p class="hint">Need beats greed, then the higher roll. Coins split evenly; the rest goes to the Party Stash.</p>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.claims {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.claim {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}
.inventory {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2,
h3 {
  margin: 0;
  font-size: 17px;
}
.containers {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 8px;
}
.container {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.container.over {
  outline: 2px dashed var(--color-gold-high);
}
.container--loot_drop {
  border-color: var(--color-gold);
}
.stack {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}
.stack input {
  width: 72px;
}
.weight,
.hint {
  color: var(--color-text-2);
  font-size: 14px;
}
.heavy {
  color: var(--color-enemy-soft);
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
