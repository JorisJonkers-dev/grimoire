<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  createSettlementMutation,
  createShopMutation,
  deleteSettlementMutation,
  deleteShopMutation,
  listLocationsOptions,
  listLootTablesOptions,
  listFactionsOptions,
  listNpcsOptions,
  listSettlementRevisionsOptions,
  listSettlementsOptions,
  listShopRevisionsOptions,
  listShopsOptions,
  rerollStockMutation,
  restoreSettlementRevisionMutation,
  restoreShopRevisionMutation,
  updateSettlementMutation,
  updateShopMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { SettlementInput, Shop, ShopInput } from '@/infrastructure/api/types.gen'
import { formatPrice } from '@/shared/coins/price'
import { GButton } from '@/shared/ui'
import RevisionHistory from './RevisionHistory.vue'
import SettlementEditor from './SettlementEditor.vue'
import ShopEditor from './ShopEditor.vue'

const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const settlements = useQuery({ ...listSettlementsOptions(path), retry: false })
const shops = useQuery({ ...listShopsOptions(path), retry: false })
const locations = useQuery({ ...listLocationsOptions(path), retry: false })
const npcs = useQuery({ ...listNpcsOptions(path), retry: false })
const factions = useQuery({ ...listFactionsOptions(path), retry: false })
const lootTables = useQuery({ ...listLootTablesOptions(path), retry: false })
const editing = ref<string | null>(null)
const history = ref<{ kind: 'settlement' | 'shop'; id: string } | null>(null)
const settlementRevisions = useQuery(
  computed(() => ({
    ...listSettlementRevisionsOptions({ path: { campaignId, settlementId: history.value?.id ?? '' } }),
    enabled: history.value?.kind === 'settlement',
  })),
)
const shopRevisions = useQuery(
  computed(() => ({ ...listShopRevisionsOptions({ path: { campaignId, shopId: history.value?.id ?? '' } }), enabled: history.value?.kind === 'shop' })),
)
const failed = ref('')
const createSettlement = useMutation(createSettlementMutation())
const updateSettlement = useMutation(updateSettlementMutation())
const removeSettlement = useMutation(deleteSettlementMutation())
const restoreSettlement = useMutation(restoreSettlementRevisionMutation())
const createShop = useMutation(createShopMutation())
const updateShop = useMutation(updateShopMutation())
const removeShop = useMutation(deleteShopMutation())
const restoreShop = useMutation(restoreShopRevisionMutation())
const reroll = useMutation(rerollStockMutation())
const fail = (what: string) => (err: unknown) => {
  const detail = (err as { detail?: string } | null)?.detail
  failed.value = detail ? `${what}: ${detail}` : `${what} failed. Try again shortly.`
}
const done = {
  onSuccess: () => {
    failed.value = ''
    editing.value = null
    void client.invalidateQueries()
  },
}
function saveSettlement(body: SettlementInput) {
  const opts = { ...done, onError: fail('Saving the settlement') }
  if (editing.value === 'new-settlement') createSettlement.mutate({ path: { campaignId }, body }, opts)
  else updateSettlement.mutate({ path: { campaignId, settlementId: editing.value ?? '' }, body }, opts)
}
function saveShop(body: ShopInput) {
  const opts = { ...done, onError: fail('Saving the shop') }
  if (editing.value === 'new-shop') createShop.mutate({ path: { campaignId }, body }, opts)
  else updateShop.mutate({ path: { campaignId, shopId: editing.value ?? '' }, body }, opts)
}
const toggle = (kind: 'settlement' | 'shop', id: string) => (history.value = history.value?.id === id ? null : { kind, id })
const shopsIn = (id: string) => (shops.data.value ?? []).filter((s) => s.settlementId === id)
const placeOf = (id?: string) => locations.data.value?.find((l) => l.id === id)?.name
const ownerOf = (s: Shop) => npcs.data.value?.find((n) => n.id === s.ownerId)?.name
const restocks = (s: Shop) => (s.restock === 'never' ? 'never restocks' : s.restock === 'long_rest' ? 'restocks after a long rest' : `restocks every ${String(s.restockDays ?? 0)} days`)
</script>

<template>
  <main class="g-page shops">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <h1>Settlements and shops</h1>
    <p v-if="settlements.isError.value" role="alert" class="g-alert" data-testid="shops-refused">Only the DM can prepare settlements and shops.</p>
    <template v-else>
      <p v-if="failed" role="alert" class="g-alert" data-testid="shops-error">{{ failed }}</p>
      <ul class="g-list">
        <li v-for="t in settlements.data.value ?? []" :key="t.id" :data-testid="`settlement-${t.name}`">
          <p>
            <strong>{{ t.name }}</strong> · {{ t.wealth }} {{ t.size }}<template v-if="placeOf(t.locationId)"> · at {{ placeOf(t.locationId) }}</template>
          </p>
          <div class="row">
            <GButton :aria-label="`Edit ${t.name}`" @click="editing = t.id">Edit</GButton>
            <GButton :aria-label="`History of ${t.name}`" @click="toggle('settlement', t.id)">History</GButton>
            <GButton
              variant="danger"
              :aria-label="`Delete ${t.name}`"
              @click="removeSettlement.mutate({ path: { campaignId, settlementId: t.id } }, { ...done, onError: fail('Deleting the settlement') })"
            >Delete</GButton>
          </div>
          <SettlementEditor v-if="editing === t.id" :settlement="t" :locations="locations.data.value ?? []" @save="saveSettlement" @cancel="editing = null" />
          <RevisionHistory
            v-if="history?.id === t.id"
            :revisions="settlementRevisions.data.value ?? []"
            :label="t.name"
            @restore="(no) => restoreSettlement.mutate({ path: { campaignId, settlementId: t.id, revisionNo: no } }, { ...done, onError: fail('Restoring the settlement') })"
          />
          <ul class="g-list shop-list">
            <li v-for="s in shopsIn(t.id)" :key="s.id" :data-testid="`shop-${s.name}`">
              <p>
                <strong>{{ s.name }}</strong> · {{ s.kind }}<template v-if="ownerOf(s)"> · kept by {{ ownerOf(s) }}</template> · +{{ s.markupPct }}% · haggle DC
                {{ s.haggleDc }} (±{{ s.hagglePct }}%) · {{ restocks(s) }}
              </p>
              <ul class="stock" :aria-label="`Stock of ${s.name}`">
                <li v-for="k in s.stock" :key="k.itemSlug" :data-testid="`stock-${s.name}-${k.itemSlug}`">{{ k.quantity }}× {{ k.itemSlug }} · {{ formatPrice(k.priceCp) }}</li>
                <li v-if="s.stock.length === 0">Nothing in stock.</li>
              </ul>
              <div class="row">
                <GButton :aria-label="`Edit ${s.name}`" @click="editing = s.id">Edit</GButton>
                <GButton
                  :aria-label="`Reroll stock of ${s.name}`"
                  :disabled="!s.lootTableId"
                  @click="reroll.mutate({ path: { campaignId, shopId: s.id } }, { ...done, onError: fail('Rerolling the stock') })"
                >Reroll stock</GButton>
                <GButton :aria-label="`History of ${s.name}`" @click="toggle('shop', s.id)">History</GButton>
                <GButton variant="danger" :aria-label="`Delete ${s.name}`" @click="removeShop.mutate({ path: { campaignId, shopId: s.id } }, { ...done, onError: fail('Deleting the shop') })">Delete</GButton>
              </div>
              <ShopEditor
                v-if="editing === s.id"
                :shop="s"
                :settlements="settlements.data.value ?? []"
                :npcs="npcs.data.value ?? []"
                :factions="factions.data.value ?? []"
                :loot-tables="lootTables.data.value ?? []"
                @save="saveShop"
                @cancel="editing = null"
              />
              <RevisionHistory
                v-if="history?.id === s.id"
                :revisions="shopRevisions.data.value ?? []"
                :label="s.name"
                @restore="(no) => restoreShop.mutate({ path: { campaignId, shopId: s.id, revisionNo: no } }, { ...done, onError: fail('Restoring the shop') })"
              />
            </li>
          </ul>
        </li>
      </ul>
      <SettlementEditor v-if="editing === 'new-settlement'" :locations="locations.data.value ?? []" @save="saveSettlement" @cancel="editing = null" />
      <ShopEditor
        v-else-if="editing === 'new-shop'"
        :settlements="settlements.data.value ?? []"
        :npcs="npcs.data.value ?? []"
        :factions="factions.data.value ?? []"
        :loot-tables="lootTables.data.value ?? []"
        @save="saveShop"
        @cancel="editing = null"
      />
      <div v-else class="row">
        <GButton data-testid="new-settlement" @click="editing = 'new-settlement'">New settlement</GButton>
        <GButton data-testid="new-shop" :disabled="(settlements.data.value ?? []).length === 0" @click="editing = 'new-shop'">New shop</GButton>
      </div>
    </template>
  </main>
</template>

<style scoped>
.shops {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.back {
  color: var(--color-gold-high);
}
p {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.shop-list {
  margin-top: 8px;
}
.stock {
  margin: 4px 0;
  padding-left: 18px;
}
</style>
