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
// The trade basket: how many of each Stock item to buy and of each carried item to sell.
const want = reactive<Record<string, number>>({})
const give = reactive<Record<string, number>>({})
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
const offerFor = (slug: string) => props.shop?.offers.find((o) => o.characterId === trader.value?.characterId && o.slug === slug)
const lines = (basket: Record<string, number>, have: { slug: string; count: number }[]) =>
  have.filter((i) => (basket[i.slug] ?? 0) > 0).map((i) => ({ itemSlug: i.slug, count: Math.min(basket[i.slug] ?? 0, i.count) }))
const buys = computed(() => lines(want, props.shop?.stock ?? []))
const sells = computed(() => lines(give, trader.value?.items ?? []))
const paying = computed(() => buys.value.reduce((sum, l) => sum + haggled(props.shop?.stock.find((k) => k.slug === l.itemSlug)?.priceCp ?? 0, adjust.value) * l.count, 0))
const getting = computed(() => sells.value.reduce((sum, l) => sum + (offerFor(l.itemSlug)?.priceCp ?? 0) * l.count, 0))
const junk = computed(() => (trader.value?.items ?? []).filter((i) => offerFor(i.slug)?.junk))
function makeTrade(sellList: { itemSlug: string; count: number }[], buyList: { itemSlug: string; count: number }[]) {
  emit('send', { kind: 'trade', fromId: buyer.value, sells: sellList, buys: buyList })
  for (const k of Object.keys(want)) want[k] = 0
  for (const k of Object.keys(give)) give[k] = 0
}
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
        <section class="trade" aria-label="Trade" data-testid="trade-window">
          <div class="side">
            <h3>{{ shop.name }} sells</h3>
            <label v-for="k in shop.stock" :key="k.slug" class="line">
              <span>{{ k.name }} · {{ formatPrice(haggled(k.priceCp, adjust)) }}</span>
              <input v-model.number="want[k.slug]" type="number" min="0" :max="k.count" class="count" :data-testid="`want-${k.slug}`" />
            </label>
          </div>
          <div class="side">
            <h3>{{ trader.label }} offers</h3>
            <p v-if="!trader.items.length" class="hint">Nothing to sell.</p>
            <label v-for="i in trader.items" :key="i.slug" class="line">
              <span>{{ i.name }} ×{{ i.count }} · <span :data-testid="`offer-${i.slug}`">{{ offerFor(i.slug) ? formatPrice(offerFor(i.slug)?.priceCp ?? 0) : 'priced at the counter' }}</span></span>
              <input v-model.number="give[i.slug]" type="number" min="0" :max="i.count" class="count" :data-testid="`give-${i.slug}`" />
            </label>
          </div>
          <p class="balance" data-testid="trade-balance">
            You pay {{ formatPrice(paying) }} and get {{ formatPrice(getting) }}:
            <strong>{{ getting >= paying ? `${formatPrice(getting - paying)} to you` : `${formatPrice(paying - getting)} from your purse` }}</strong>
          </p>
          <p class="row">
            <GButton variant="primary" :disabled="buys.length + sells.length === 0" data-testid="make-trade" @click="makeTrade(sells, buys)">Make the trade</GButton>
            <GButton
              v-if="junk.length"
              data-testid="sell-junk"
              @click="makeTrade(junk.map((i) => ({ itemSlug: i.slug, count: i.count })), [])"
            >Sell the junk</GButton>
          </p>
        </section>
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
.trade {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 8px 16px;
}
.trade h3 {
  margin: 0 0 4px;
  font-size: 15px;
}
.side {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.balance,
.trade .row {
  grid-column: 1 / -1;
}
.hint {
  color: var(--color-text-2);
}
</style>
