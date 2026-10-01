<script setup lang="ts">
import { computed, reactive, ref, watchEffect } from 'vue'
import type { LiveContainer, LiveShop, Shop } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { formatPrice, haggled } from '@/shared/coins/price'
import { GButton } from '@/shared/ui'
import LiveRoll from './LiveRoll.vue'

const props = defineProps<{ shop?: LiveShop; shops: Shop[]; containers: LiveContainer[]; dm: boolean; me: string; campaignId: string; gameDay: number }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()
const choice = ref('')
const buyer = ref('')
const buying = reactive<Record<string, number>>({})
const selling = ref('')
const sellCount = ref(1)
// The DM trades for any Character; a player for their own.
const traders = computed(() => props.containers.filter((c) => c.kind === 'character' && c.characterId && (props.dm || c.ownerId === props.me)))
watchEffect(() => {
  if (!traders.value.some((c) => c.id === buyer.value)) buyer.value = traders.value[0]?.id ?? ''
})
const trader = computed(() => traders.value.find((c) => c.id === buyer.value))
const haggle = computed(() => props.shop?.haggles.find((h) => h.characterId === trader.value?.characterId))
const adjust = computed(() => haggle.value?.adjustPct ?? 0)
// A haggle's Roll Card goes to whoever owns the Character.
const myRolls = computed(() =>
  (props.shop?.haggles ?? []).filter((h) => h.rollId && h.adjustPct === undefined && props.containers.some((c) => c.characterId === h.characterId && c.ownerId === props.me)),
)
const outcome = (pct: number) => (pct < 0 ? `${String(-pct)}% off` : pct > 0 ? `${String(pct)}% dearer` : 'no change')
</script>

<template>
  <section class="g-card shop" aria-label="Shop" data-testid="shop-panel">
    <h2>Shop <span class="day" data-testid="game-day">Day {{ gameDay }}</span></h2>
    <div v-if="dm" class="row">
      <template v-if="!shop">
        <label class="g-field grow">
          <span>Shop</span>
          <select v-model="choice" data-testid="shop-choice">
            <option value="" disabled>Choose a shop</option>
            <option v-for="s in shops" :key="s.id" :value="s.id">{{ s.name }}</option>
          </select>
        </label>
        <GButton :disabled="!choice" data-testid="open-shop" @click="emit('send', { kind: 'open_shop', shopId: choice })">Open shop</GButton>
      </template>
      <GButton v-else data-testid="close-shop" @click="emit('send', { kind: 'close_shop' })">Close shop</GButton>
    </div>
    <template v-if="shop">
      <p data-testid="shop-open">
        <strong>{{ shop.name }}</strong> · {{ shop.kind }} in {{ shop.settlement }}<template v-if="shop.owner"> · kept by {{ shop.owner }}</template>
      </p>
      <label v-if="traders.length > 1" class="g-field">
        <span>Trading as</span>
        <select v-model="buyer" data-testid="shop-buyer">
          <option v-for="c in traders" :key="c.id" :value="c.id">{{ c.label }}</option>
        </select>
      </label>
      <template v-if="trader">
        <p class="row">
          <span v-if="haggle?.adjustPct !== undefined" data-testid="haggle-result">Haggled: {{ outcome(haggle.adjustPct) }}</span>
          <span v-else-if="haggle" data-testid="haggle-pending">Haggling…</span>
          <GButton v-else data-testid="haggle" @click="emit('send', { kind: 'haggle', fromId: buyer })">Haggle</GButton>
        </p>
        <ul class="g-list" :aria-label="`Stock of ${shop.name}`">
          <li v-for="k in shop.stock" :key="k.slug" class="row" :data-testid="`stock-${k.slug}`">
            <span>{{ k.name }} ×{{ k.count }} · <span data-testid="price">{{ formatPrice(haggled(k.priceCp, adjust)) }}</span></span>
            <input v-model.number="buying[k.slug]" type="number" min="1" :max="k.count" :aria-label="`How many ${k.name}`" class="count" />
            <GButton
              :aria-label="`Buy ${k.name}`"
              :data-testid="`buy-${k.slug}`"
              @click="emit('send', { kind: 'buy', fromId: buyer, itemSlug: k.slug, count: buying[k.slug] ?? 1 })"
            >Buy</GButton>
          </li>
          <li v-if="shop.stock.length === 0">Sold out.</li>
        </ul>
        <div v-if="trader.items.length" class="row">
          <label class="g-field grow">
            <span>Sell</span>
            <select v-model="selling" data-testid="sell-item">
              <option value="" disabled>Choose an item</option>
              <option v-for="i in trader.items" :key="i.slug" :value="i.slug">{{ i.name }} ×{{ i.count }}</option>
            </select>
          </label>
          <label class="g-field"><span>How many</span><input v-model.number="sellCount" type="number" min="1" data-testid="sell-count" /></label>
          <GButton :disabled="!selling" data-testid="sell" @click="emit('send', { kind: 'sell', fromId: buyer, itemSlug: selling, count: sellCount })">Sell for half</GButton>
        </div>
      </template>
      <LiveRoll v-for="h in myRolls" :key="h.rollId" :campaign-id="campaignId" :roll-id="h.rollId ?? ''" />
    </template>
  </section>
</template>

<style scoped>
.shop {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
}
.day {
  font-size: 14px;
  color: var(--color-text-2);
}
p {
  margin: 0;
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
.count {
  width: 64px;
}
</style>
