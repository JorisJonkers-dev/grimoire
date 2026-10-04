<script setup lang="ts">
import CampaignFrame from './CampaignFrame.vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  createFactionMutation,
  decideStandingChangeMutation,
  deleteFactionMutation,
  getCampaignOptions,
  listCharactersOptions,
  listFactionArchetypesOptions,
  listFactionsOptions,
  proposeStandingChangeMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Faction, StandingChange, StandingTier } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

// Factions and how each regards the party. Everyone sees tiers and the reasons the DM shared; the DM
// keeps the Factions, sees the score behind each tier, and decides every Standing Change that waits.
const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const campaign = useQuery({ ...getCampaignOptions(path), retry: false })
const dm = computed(() => campaign.data.value?.myRole === 'dm')
const list = useQuery({ ...listFactionsOptions(path), retry: false })
const factions = computed(() => list.data.value ?? [])
const catalogue = useQuery({ ...listFactionArchetypesOptions(), retry: false })
const archetypes = computed(() => catalogue.data.value ?? [])
const characters = useQuery(computed(() => ({ ...listCharactersOptions(path), enabled: dm.value })))

const tiers: Record<StandingTier, string> = { hostile: 'Hostile', unfriendly: 'Unfriendly', neutral: 'Neutral', friendly: 'Friendly', allied: 'Allied' }
const archetypeName = (slug: string) => archetypes.value.find((a) => a.slug === slug)?.name ?? ''
const problem = ref('')
const failed = () => { problem.value = 'That could not be done. Check what you entered and try again.' }
const done = () => {
  problem.value = ''
  void client.invalidateQueries()
}

// What a change says: to a Player, which way it went and the reason if it was shared; to the DM, all of it.
function says(ch: StandingChange): string {
  const way = ch.rose ? 'rose' : 'fell'
  if (!ch.dm) return ch.reason ? `Standing ${way}: ${ch.reason}` : `Standing ${way}.`
  const amount = String(Math.abs(ch.dm.delta))
  if (ch.status === 'pending') return `Suggested by ${ch.dm.client || (ch.dm.origin === 'ui' ? 'you' : ch.dm.origin)}: ${way} by ${amount}. ${ch.reason}`
  if (ch.status === 'dismissed') return `Dismissed: ${way} by ${amount}. ${ch.reason}`
  return `Standing ${way} by ${amount}: ${ch.reason} ${ch.dm.shareReason ? 'Shared with the Players.' : 'Not shared.'}`
}

// A new Faction, from an archetype of the catalogue or from nothing.
const draft = reactive({ name: '', archetype: '', goals: '', territory: '', notes: '' })
function pick() {
  const a = archetypes.value.find((x) => x.slug === draft.archetype)
  if (a) Object.assign(draft, { name: a.name, goals: a.goals })
}
const create = useMutation(createFactionMutation())
function add() {
  create.mutate({ ...path, body: { ...draft, name: draft.name.trim() } }, {
    onSuccess: () => {
      Object.assign(draft, { name: '', archetype: '', goals: '', territory: '', notes: '' })
      done()
    },
    onError: failed,
  })
}
const remove = useMutation(deleteFactionMutation())

// A suggested Standing Change, for the party or for one Character; it waits for the DM's word.
const suggestion = reactive<Record<string, { delta: number | ''; reason: string; share: boolean; who: string }>>({})
const suggestionFor = (id: string) => (suggestion[id] ??= { delta: '', reason: '', share: false, who: '' })
const who = (f: Faction) => {
  const known = new Map(f.personal.map((p) => [p.characterId, p.character]))
  for (const c of characters.data.value ?? []) known.set(c.id, c.name)
  return [...known].map(([id, name]) => ({ id, name }))
}
const propose = useMutation(proposeStandingChangeMutation())
function suggest(f: Faction) {
  const s = suggestionFor(f.id)
  propose.mutate({ path: { campaignId, factionId: f.id }, body: { delta: Number(s.delta), reason: s.reason.trim(), shareReason: s.share, ...(s.who ? { characterId: s.who } : {}) } }, {
    onSuccess: () => {
      Object.assign(s, { delta: '', reason: '', share: false, who: '' })
      done()
    },
    onError: failed,
  })
}

// The DM's word on a change that waits: confirmed as suggested or edited, or dismissed.
const edits = reactive<Record<string, { delta: number; reason: string; share: boolean }>>({})
const editOf = (ch: StandingChange) => (edits[ch.id] ??= { delta: ch.dm?.delta ?? 0, reason: ch.reason, share: ch.dm?.shareReason ?? false })
const decision = useMutation(decideStandingChangeMutation())
function decide(ch: StandingChange, confirm: boolean) {
  const e = editOf(ch)
  const body = confirm ? { confirm, delta: e.delta, reason: e.reason.trim(), shareReason: e.share } : { confirm }
  decision.mutate({ path: { campaignId, changeId: ch.id }, body }, { onSuccess: done, onError: failed })
}
</script>

<template>
  <main class="g-page">
    <CampaignFrame current="factions">
      <header class="g-headline">
        <span class="g-eyebrow">Campaign</span>
        <h1>Factions</h1>
      </header>
    </CampaignFrame>
    <p v-if="list.isError.value" role="alert" class="g-alert" data-testid="factions-missing">That Campaign is not available.</p>
    <template v-else>
      <p class="hint">How each Faction regards the party, from Hostile to Allied. The DM decides when it changes.</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="faction-problem">{{ problem }}</p>
      <p v-if="list.isSuccess.value && factions.length === 0" class="hint" data-testid="no-factions">No Factions in this Campaign yet.</p>
      <section v-for="f in factions" :key="f.id" class="g-card faction" :aria-labelledby="`faction-title-${f.id}`" :data-testid="`faction-${f.id}`">
        <header class="row">
          <h2 :id="`faction-title-${f.id}`">{{ f.name }}</h2>
          <span :class="['tier', `tier--${f.tier}`]" data-testid="tier">{{ tiers[f.tier] }}</span>
        </header>
        <p v-if="f.archetype" class="hint" data-testid="archetype">{{ archetypeName(f.archetype) }}</p>
        <dl v-if="f.dm" class="secrets" data-testid="faction-secrets">
          <dt>For the DM</dt>
          <dd>Score {{ f.dm.score }}</dd>
          <dd v-if="f.dm.goals">Goals: {{ f.dm.goals }}</dd>
          <dd v-if="f.dm.territory">Territory: {{ f.dm.territory }}</dd>
          <dd v-if="f.dm.notes">Notes: {{ f.dm.notes }}</dd>
        </dl>
        <ul v-if="f.personal.length > 0" class="g-list" aria-label="Personal Standing">
          <li v-for="p in f.personal" :key="p.characterId" data-testid="personal">{{ p.character }}: {{ tiers[p.tier] }}{{ p.score === undefined ? '' : ` (${String(p.score)})` }}</li>
        </ul>
        <ul v-if="f.changes.length > 0" class="g-list" aria-label="Standing Changes">
          <li v-for="ch in f.changes" :key="ch.id" :data-testid="`change-${ch.id}`">
            <span>{{ says(ch) }}</span>
            <div v-if="ch.dm && ch.status === 'pending'" class="row decide" data-testid="decide">
              <label class="g-field"><span>Change by</span><input v-model.number="editOf(ch).delta" type="number" min="-100" max="100" data-testid="decide-delta" /></label>
              <label class="g-field grow"><span>Reason</span><input v-model="editOf(ch).reason" maxlength="500" data-testid="decide-reason" /></label>
              <label class="check"><input v-model="editOf(ch).share" type="checkbox" data-testid="decide-share" /><span>Players see the reason</span></label>
              <GButton variant="primary" data-testid="decide-confirm" @click="decide(ch, true)">Confirm</GButton>
              <GButton data-testid="decide-dismiss" @click="decide(ch, false)">Dismiss</GButton>
            </div>
          </li>
        </ul>
        <template v-if="dm">
          <form class="row" :aria-label="`Suggest a Standing Change for ${f.name}`" data-testid="suggest" @submit.prevent="suggest(f)">
            <label class="g-field"><span>Change by</span><input v-model.number="suggestionFor(f.id).delta" type="number" min="-100" max="100" data-testid="suggest-delta" /></label>
            <label class="g-field grow"><span>Reason</span><input v-model="suggestionFor(f.id).reason" maxlength="500" data-testid="suggest-reason" /></label>
            <label class="g-field">
              <span>For</span>
              <select v-model="suggestionFor(f.id).who" data-testid="suggest-who">
                <option value="">The party</option>
                <option v-for="c in who(f)" :key="c.id" :value="c.id">{{ c.name }}</option>
              </select>
            </label>
            <label class="check"><input v-model="suggestionFor(f.id).share" type="checkbox" data-testid="suggest-share" /><span>Players see the reason</span></label>
            <GButton type="submit" :disabled="!suggestionFor(f.id).delta || suggestionFor(f.id).reason.trim() === ''">Suggest</GButton>
          </form>
          <GButton variant="danger" :aria-label="`Remove ${f.name}`" data-testid="faction-remove" @click="remove.mutate({ path: { campaignId, factionId: f.id } }, { onSuccess: done, onError: failed })">Remove</GButton>
        </template>
      </section>
      <form v-if="dm" class="g-card faction" aria-label="Add a Faction" data-testid="faction-add" @submit.prevent="add()">
        <h2>Add a Faction</h2>
        <label class="g-field">
          <span>Start from an archetype</span>
          <select v-model="draft.archetype" data-testid="faction-archetype" @change="pick()">
            <option value="">None</option>
            <option v-for="a in archetypes" :key="a.slug" :value="a.slug">{{ a.name }}</option>
          </select>
        </label>
        <label class="g-field"><span>Name</span><input v-model="draft.name" maxlength="80" data-testid="faction-name" /></label>
        <label class="g-field"><span>Goals</span><textarea v-model="draft.goals" maxlength="2000" rows="2" data-testid="faction-goals"></textarea></label>
        <label class="g-field"><span>Territory</span><input v-model="draft.territory" maxlength="2000" data-testid="faction-territory" /></label>
        <label class="g-field"><span>Notes for you alone</span><textarea v-model="draft.notes" maxlength="2000" rows="2" data-testid="faction-notes"></textarea></label>
        <GButton type="submit" variant="primary" :disabled="draft.name.trim() === ''">Add Faction</GButton>
      </form>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.faction {
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
.row h2 {
  margin: 0;
  flex: 1;
}
.grow {
  flex: 1;
  min-width: 180px;
}
.tier {
  padding: 2px 10px;
  border-radius: 999px;
  border: 1px solid var(--color-line);
  font-weight: 700;
}
.tier--hostile,
.tier--unfriendly {
  border-color: var(--color-enemy);
}
.tier--friendly,
.tier--allied {
  border-color: var(--color-party);
}
.secrets {
  margin: 0;
}
.secrets dt {
  font-weight: 700;
}
.secrets dd {
  margin: 0;
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
}
.decide {
  margin-top: 6px;
}
</style>
