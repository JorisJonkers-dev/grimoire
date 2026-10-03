<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { createRollMutation, getActionLogOptions, getCampaignOptions, listRollsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { RollModifier, RollRequest } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import DiceHost from '@/features/dice/DiceHost.vue'
import RollCard from './RollCard.vue'
import { dieSizes, notationFor, type Throw } from './notation'

const route = useRoute()
const client = useQueryClient()
const id = computed(() => String(route.params.id))
const path = computed(() => ({ path: { campaignId: id.value } }))
const campaign = useQuery(computed(() => ({ ...getCampaignOptions(path.value), retry: false })))
const isDM = computed(() => campaign.data.value?.myRole === 'dm')
const rolls = useQuery(computed(() => ({ ...listRollsOptions({ ...path.value, query: { limit: 20 } }), enabled: campaign.isSuccess.value })))
const log = useQuery(computed(() => ({ ...getActionLogOptions({ ...path.value, query: { limit: 30 } }), enabled: isDM.value })))

const purpose = ref('')
const t = reactive<Throw>({ count: 1, faces: 20, edge: 'normal', bless: false, bane: false })
const modifiers = ref<RollModifier[]>([])
const label = ref('')
const value = ref(0)
const built = computed(() => notationFor(t))
const current = ref<RollRequest | null>(null)
const create = useMutation(createRollMutation())

function addModifier() {
  if (label.value.trim() === '') return
  modifiers.value = [...modifiers.value, { label: label.value.trim(), value: value.value }]
  label.value = ''
  value.value = 0
}
function request() {
  create.mutate(
    { ...path.value, body: { purpose: purpose.value.trim(), notation: built.value.notation, labels: built.value.labels, modifiers: modifiers.value } },
    {
      onSuccess: (r) => {
        current.value = r
        void client.invalidateQueries()
      },
    },
  )
}
function updated(r: RollRequest) {
  current.value = r
  void client.invalidateQueries()
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id } }" class="back">← Campaign</RouterLink>
    <h1>Dice</h1>
    <p v-if="campaign.isError.value" role="alert" class="g-alert" data-testid="dice-missing">This campaign does not exist, or you are not one of its members.</p>
    <template v-else>
      <form class="g-card build" data-testid="roll-form" @submit.prevent="request">
        <label class="g-field"><span>What is this roll for?</span><input v-model="purpose" maxlength="120" placeholder="Stealth check" data-testid="roll-purpose" /></label>
        <div class="row">
          <label class="g-field"><span>Dice</span><input v-model.number="t.count" type="number" min="1" max="20" data-testid="roll-count" /></label>
          <label class="g-field">
            <span>Die</span>
            <select v-model.number="t.faces" data-testid="roll-faces">
              <option v-for="f in dieSizes" :key="f" :value="f">d{{ f }}</option>
            </select>
          </label>
          <label class="g-field">
            <span>Edge</span>
            <select v-model="t.edge" :disabled="t.faces !== 20 || t.count !== 1" data-testid="roll-edge">
              <option value="normal">Normal</option>
              <option value="advantage">Advantage</option>
              <option value="disadvantage">Disadvantage</option>
            </select>
          </label>
        </div>
        <div class="row">
          <label class="check"><input v-model="t.bless" type="checkbox" data-testid="roll-bless" /><span>Bless (+1d4)</span></label>
          <label class="check"><input v-model="t.bane" type="checkbox" /><span>Bane (−1d4)</span></label>
        </div>
        <div class="row">
          <label class="g-field grow"><span>Modifier</span><input v-model="label" maxlength="80" placeholder="Dexterity" data-testid="mod-label" /></label>
          <label class="g-field"><span>Value</span><input v-model.number="value" type="number" min="-100" max="100" data-testid="mod-value" /></label>
          <GButton data-testid="mod-add" @click="addModifier()">Add</GButton>
        </div>
        <ul v-if="modifiers.length" class="mods">
          <li v-for="(m, i) in modifiers" :key="i">{{ m.label }} {{ m.value >= 0 ? '+' : '' }}{{ m.value }}</li>
        </ul>
        <p class="hint">Throws <code data-testid="roll-notation">{{ built.notation }}</code></p>
        <GButton type="submit" variant="primary" :disabled="purpose.trim() === '' || create.isPending.value">Ask for the roll</GButton>
        <p v-if="create.isError.value" role="alert" class="g-alert">That roll could not be made.</p>
      </form>

      <RollCard v-if="current" :key="current.id" :roll="current" :campaign-id="id" @updated="updated" />

      <section class="g-card">
        <h2>Recent rolls</h2>
        <ul class="g-list" data-testid="roll-history">
          <li v-for="r in rolls.data.value ?? []" :key="r.id" class="past">
            <span>{{ r.purpose }} · {{ r.roller.name }} · <code>{{ r.notation }}</code></span>
            <strong v-if="r.status === 'resolved'">{{ r.total }}</strong>
            <GButton v-else-if="r.canRoll" @click="current = r">Roll it</GButton>
            <span v-else class="hint">waiting</span>
          </li>
        </ul>
      </section>

      <section v-if="isDM" class="g-card" data-testid="action-log">
        <h2>Action Log</h2>
        <ol class="log">
          <li v-for="a in log.data.value ?? []" :key="a.seq">
            #{{ a.seq }} {{ a.actor }} {{ a.kind.replace('_', ' ') }}<template v-if="a.dieNo !== undefined"> die {{ a.dieNo + 1 }}</template>:
            {{ a.value }}<template v-if="a.seed"> <small>seed {{ a.seed }}</small></template>
          </li>
        </ol>
      </section>
    </template>
    <DiceHost />
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.build {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.row .g-field {
  min-width: 90px;
}
.grow {
  flex: 1 1 160px;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.mods {
  margin: 0;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
h2 {
  margin: 0 0 8px;
  font-family: var(--font-display);
  font-size: 17px;
}
.past {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 44px;
}
.log {
  margin: 0;
  padding-left: 20px;
  font-size: 14px;
  color: var(--color-text-2);
}
</style>
