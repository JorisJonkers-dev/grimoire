<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { endSessionMutation, getCampaignOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { TokenKind } from '@/infrastructure/api/types.gen'
import { useLiveSession, type SocketFactory } from '@/realtime/liveSession'
import type { Coord } from '@/shared/hex'
import HexGrid from '@/shared/map/HexGrid.vue'
import { GButton } from '@/shared/ui'
import { board } from './board'

const props = defineProps<{ socket?: SocketFactory }>()
const route = useRoute()
const router = useRouter()
const campaignId = String(route.params.id)
const sessionId = String(route.params.sid)
const campaign = useQuery({ ...getCampaignOptions({ path: { campaignId } }), retry: false })
const isDM = computed(() => campaign.data.value?.myRole === 'dm')
const live = ref<ReturnType<typeof useLiveSession> | null>(null)
const view = computed(() => live.value?.view)

const connect = (dm: boolean) => {
  live.value ??= useLiveSession(campaignId, sessionId, dm ? 'dm' : 'party', props.socket)
}
const ready = computed(() => {
  if (campaign.data.value) connect(isDM.value)
  return Boolean(live.value)
})

const selected = ref<string | null>(null)
const label = ref('')
const kind = ref<TokenKind>('enemy')
const hidden = ref(false)
const cells = computed(() => board(view.value?.session?.gridRadius ?? 0, view.value?.tokens ?? [], selected.value))
const chosen = computed(() => view.value?.tokens.find((t) => t.id === selected.value) ?? null)

function pick(c: Coord) {
  if (!isDM.value || !live.value) return
  const there = view.value?.tokens.find((t) => t.q === c.q && t.r === c.r)
  if (there) {
    selected.value = there.id === selected.value ? null : there.id
    return
  }
  if (chosen.value) {
    live.value.send({ kind: 'move_token', tokenId: chosen.value.id, q: c.q, r: c.r })
  } else if (label.value.trim()) {
    live.value.send({ kind: 'place_token', label: label.value.trim(), tokenKind: kind.value, q: c.q, r: c.r, hidden: hidden.value })
    label.value = ''
  }
}
function toggleHidden() {
  if (chosen.value) live.value?.send({ kind: 'set_token_hidden', tokenId: chosen.value.id, hidden: !chosen.value.hidden })
}
function remove() {
  if (chosen.value) live.value?.send({ kind: 'remove_token', tokenId: chosen.value.id })
  selected.value = null
}
const end = useMutation(endSessionMutation())
function endSession() {
  end.mutate({ path: { campaignId, sessionId } }, { onSuccess: () => void router.push({ name: 'campaign', params: { id: campaignId } }) })
}
const status = computed(() => ({ connecting: 'Connecting…', open: 'Live', reconnecting: 'Reconnecting…', ended: 'Session ended' })[view.value?.connection ?? 'connecting'])
</script>

<template>
  <main class="g-page live">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <p v-if="campaign.isError.value" role="alert" class="g-alert" data-testid="live-missing">This session is not available to you.</p>
    <template v-else-if="ready && view">
      <header class="head">
        <h1>Session {{ view.session?.number ?? '' }}</h1>
        <span class="g-tag" :class="`conn--${view.connection}`" data-testid="connection" role="status">{{ status }}</span>
        <RouterLink :to="{ name: 'table', params: { id: campaignId, sid: sessionId } }" class="table-link">Table display</RouterLink>
      </header>
      <p v-if="view.rejection" role="alert" class="g-alert" data-testid="rejection">{{ view.rejection }}</p>
      <HexGrid :cells="cells" :title="`Session ${String(view.session?.number ?? '')} map`" @select="pick" />
      <section v-if="isDM" class="g-card controls" data-testid="dm-controls">
        <h2>Tokens</h2>
        <p class="hint">Name a token and tap an empty hex to place it. Tap a token to select it, then an empty hex to move it.</p>
        <div class="row">
          <label class="g-field grow"><span>Name</span><input v-model="label" maxlength="40" data-testid="token-label" /></label>
          <label class="g-field">
            <span>Kind</span>
            <select v-model="kind" data-testid="token-kind">
              <option value="party">Party</option>
              <option value="enemy">Enemy</option>
              <option value="npc">NPC</option>
              <option value="object">Object</option>
            </select>
          </label>
          <label class="check"><input v-model="hidden" type="checkbox" data-testid="token-hidden" /><span>Hidden</span></label>
        </div>
        <div v-if="chosen" class="row" data-testid="selected-token">
          <span>{{ chosen.label }}{{ chosen.hidden ? ' (hidden)' : '' }}</span>
          <GButton data-testid="toggle-hidden" @click="toggleHidden()">{{ chosen.hidden ? 'Reveal' : 'Hide' }}</GButton>
          <GButton variant="danger" data-testid="remove-token" @click="remove()">Remove</GButton>
        </div>
        <GButton variant="danger" data-testid="end-session" @click="endSession()">End session</GButton>
      </section>
      <ul class="g-list tokens" aria-label="Tokens on the map">
        <li v-for="t in view.tokens" :key="t.id">{{ t.label }} · {{ t.kind }}{{ t.hidden ? ' · hidden' : '' }}</li>
      </ul>
    </template>
    <p v-else>Opening the session…</p>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.conn--open {
  border-color: var(--color-success);
  color: var(--color-success);
}
.conn--reconnecting,
.conn--ended {
  border-color: var(--color-enemy);
  color: var(--color-enemy-soft);
}
.table-link {
  margin-left: auto;
  color: var(--color-gold-high);
}
.controls {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
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
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.tokens {
  font-size: 14px;
  color: var(--color-text-2);
}
</style>
