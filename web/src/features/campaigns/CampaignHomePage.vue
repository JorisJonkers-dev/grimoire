<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createInviteMutation,
  getCampaignOptions,
  listCharactersOptions,
  listSessionsOptions,
  startSessionMutation,
  listInvitesOptions,
  removeMemberMutation,
  revokeInviteMutation,
  updateCampaignMutation,
  updateMemberMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Member, Role } from '@/infrastructure/api/types.gen'
import { GButton, TokenBadge } from '@/shared/ui'
import CompanionsSection from './CompanionsSection.vue'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const id = computed(() => String(route.params.id))
const path = computed(() => ({ path: { campaignId: id.value } }))

const campaign = useQuery(computed(() => ({ ...getCampaignOptions(path.value), retry: false })))
const isDM = computed(() => campaign.data.value?.myRole === 'dm')
const sessions = useQuery(computed(() => ({ ...listSessionsOptions(path.value), enabled: campaign.isSuccess.value })))
const liveSessions = computed(() => (sessions.data.value ?? []).filter((s) => s.status === 'live'))
const start = useMutation(startSessionMutation())
function startSession() {
  start.mutate(path.value, {
    onSuccess: (s) => void router.push({ name: 'session', params: { id: id.value, sid: s.id } }),
    onError: onError('The session could not be started.'),
  })
}
const characters = useQuery(computed(() => ({ ...listCharactersOptions(path.value), enabled: campaign.isSuccess.value })))
const invites = useQuery(computed(() => ({ ...listInvitesOptions(path.value), enabled: isDM.value })))

const refresh = () => void client.invalidateQueries()
const reactionTimeout = ref<number | null>(null)
const timeout = computed({
  get: () => reactionTimeout.value ?? campaign.data.value?.reactionTimeoutS ?? 10,
  set: (v: number) => (reactionTimeout.value = v),
})
const suppliesChoice = ref<boolean | null>(null)
const restSupplies = computed({
  get: () => suppliesChoice.value ?? campaign.data.value?.restSupplies ?? false,
  set: (v: boolean) => (suppliesChoice.value = v),
})
const highGroundChoice = ref<boolean | null>(null)
const highGround = computed({
  get: () => highGroundChoice.value ?? campaign.data.value?.highGround ?? false,
  set: (v: boolean) => (highGroundChoice.value = v),
})
const modeChoice = ref<'individual' | 'side' | null>(null)
const initiativeMode = computed({
  get: () => modeChoice.value ?? campaign.data.value?.initiativeMode ?? 'individual',
  set: (v: 'individual' | 'side') => (modeChoice.value = v),
})
const shareChoice = ref<boolean | null>(null)
const shareInitiative = computed({
  get: () => shareChoice.value ?? campaign.data.value?.shareInitiative ?? false,
  set: (v: boolean) => (shareChoice.value = v),
})
type CreationMethod = 'standard-array' | 'point-buy' | 'rolled'
const methodsChoice = ref<CreationMethod[] | null>(null)
const creationMethods = computed({
  get: () => methodsChoice.value ?? campaign.data.value?.creationMethods ?? ['standard-array', 'point-buy', 'rolled'],
  set: (v: CreationMethod[]) => (methodsChoice.value = v),
})
const levelChoice = ref<number | null>(null)
const startingLevel = computed({
  get: () => levelChoice.value ?? campaign.data.value?.startingLevel ?? 1,
  set: (v: number) => (levelChoice.value = v),
})
const holdChoice = ref<boolean | null>(null)
const holdLevelUps = computed({
  get: () => holdChoice.value ?? campaign.data.value?.holdLevelUps ?? false,
  set: (v: boolean) => (holdChoice.value = v),
})
const noUndoChoice = ref<boolean | null>(null)
const noUndo = computed({
  get: () => noUndoChoice.value ?? campaign.data.value?.noUndo ?? false,
  set: (v: boolean) => (noUndoChoice.value = v),
})
const showDcsChoice = ref<boolean | null>(null)
const showDcs = computed({
  get: () => showDcsChoice.value ?? campaign.data.value?.showDcs ?? false,
  set: (v: boolean) => (showDcsChoice.value = v),
})
const exhaustionChoice = ref<string | null>(null)
const exhaustion = computed({
  get: () => exhaustionChoice.value ?? campaign.data.value?.exhaustion ?? 'srd-2024',
  set: (v: string) => (exhaustionChoice.value = v),
})
const settings = useMutation(updateCampaignMutation())
function saveSettings() {
  failed.value = ''
  settings.mutate({ ...path.value, body: {
    reactionTimeoutS: timeout.value, highGround: highGround.value, restSupplies: restSupplies.value, initiativeMode: initiativeMode.value,
    shareInitiative: shareInitiative.value, creationMethods: creationMethods.value, startingLevel: startingLevel.value,
    holdLevelUps: holdLevelUps.value, noUndo: noUndo.value, showDcs: showDcs.value, exhaustion: exhaustion.value,
  } }, { onSuccess: refresh, onError: onError('The settings could not be saved.') })
}
const failed = ref('')
const onError = (what: string) => () => (failed.value = what)

const newLink = ref('')
const copied = ref(false)
const createInvite = useMutation(createInviteMutation())
const revoke = useMutation(revokeInviteMutation())
const setRole = useMutation(updateMemberMutation())
const remove = useMutation(removeMemberMutation())

function inviteLink() {
  failed.value = ''
  createInvite.mutate(path.value, {
    onSuccess: (inv) => {
      newLink.value = `${window.location.origin}/join#${inv.token}`
      copied.value = false
      refresh()
    },
    onError: onError('The invite link could not be created.'),
  })
}
function revokeInvite(inviteId: string) {
  failed.value = ''
  revoke.mutate({ path: { campaignId: id.value, inviteId } }, { onSuccess: refresh, onError: onError('The invite could not be revoked.') })
}
function changeRole(m: Member, role: Role) {
  failed.value = ''
  setRole.mutate(
    { path: { campaignId: id.value, memberId: m.id }, body: { role } },
    { onSuccess: refresh, onError: onError('That role change was refused. A campaign needs at least one DM.') },
  )
}
function removeMember(m: Member) {
  failed.value = ''
  remove.mutate(
    { path: { campaignId: id.value, memberId: m.id } },
    {
      onSuccess: () => {
        if (m.isMe) void router.push({ name: 'campaigns' })
        else refresh()
      },
      onError: onError('That member could not be removed. A campaign needs at least one DM.'),
    },
  )
}
async function copy() {
  await navigator.clipboard.writeText(newLink.value)
  copied.value = true
}
const expires = (iso: string) => new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaigns' }" class="back">← Campaigns</RouterLink>
    <p v-if="campaign.isPending.value">Opening the campaign…</p>
    <p v-else-if="campaign.isError.value" role="alert" class="g-alert" data-testid="campaign-missing">
      This campaign does not exist, or you are not one of its members.
    </p>
    <template v-else-if="campaign.data.value">
      <header>
        <h1>{{ campaign.data.value.name }}</h1>
        <p class="sub">
          SRD 5.2 · you are
          {{ isDM ? 'a DM' : 'a Player' }}
        </p>
      </header>
      <p v-if="failed" role="alert" class="g-alert" data-testid="campaign-error">{{ failed }}</p>
      <nav class="prep" aria-label="Campaign tools">
        <RouterLink :to="{ name: 'dice', params: { id } }" data-testid="dice-link">Dice</RouterLink>
        <RouterLink :to="{ name: 'proposals', params: { id } }" data-testid="proposals-link">Proposals</RouterLink>
        <RouterLink v-if="isDM" :to="{ name: 'npcs', params: { id } }" data-testid="npcs-link">NPCs</RouterLink>
        <RouterLink v-if="isDM" :to="{ name: 'campaign-library', params: { id } }" data-testid="library-link">Library</RouterLink>
        <RouterLink v-if="isDM" :to="{ name: 'maps', params: { id } }" data-testid="maps-link">Maps</RouterLink>
        <RouterLink :to="{ name: 'journal', params: { id } }" data-testid="journal-link">Journal</RouterLink>
        <RouterLink :to="{ name: 'factions', params: { id } }" data-testid="factions-link">Factions</RouterLink>
        <RouterLink :to="{ name: 'rule-variants', params: { id } }" data-testid="rules-link">Rule Variants</RouterLink>
        <RouterLink :to="{ name: 'tracks', params: { id } }" data-testid="tracks-link">Tracks</RouterLink>
        <RouterLink :to="{ name: 'downtime', params: { id } }" data-testid="downtime-link">Downtime</RouterLink>
        <RouterLink :to="{ name: 'vehicles', params: { id } }" data-testid="vehicles-link">Vehicles</RouterLink>
        <RouterLink v-if="isDM" :to="{ name: 'encounters', params: { id } }" data-testid="encounters-link">Random encounters</RouterLink>
        <RouterLink v-if="isDM" :to="{ name: 'loot', params: { id } }" data-testid="loot-link">Loot tables</RouterLink>
        <RouterLink v-if="isDM" :to="{ name: 'shops', params: { id } }" data-testid="shops-link">Settlements and shops</RouterLink>
        <RouterLink v-if="isDM" :to="{ name: 'activity', params: { id } }" data-testid="activity-link">AI activity</RouterLink>
      </nav>

      <section class="g-card" data-testid="sessions">
        <h2>Sessions</h2>
        <ul class="g-list">
          <li v-for="s in liveSessions" :key="s.id" class="member">
            <span class="who">Session {{ s.number }} is live</span>
            <RouterLink :to="{ name: 'session', params: { id, sid: s.id } }" data-testid="join-session">Join</RouterLink>
            <RouterLink :to="{ name: 'table', params: { id, sid: s.id } }">Table display</RouterLink>
          </li>
        </ul>
        <p v-if="liveSessions.length === 0" class="hint">No session is running.</p>
        <GButton v-if="isDM" variant="primary" data-testid="start-session" @click="startSession()">Start a session</GButton>
      </section>

      <section class="g-card" data-testid="party">
        <h2>Party</h2>
        <p v-if="(characters.data.value ?? []).length === 0" class="hint">No characters yet.</p>
        <ul class="g-list">
          <li v-for="ch in characters.data.value ?? []" :key="ch.id">
            <RouterLink :to="{ name: 'character', params: { id, characterId: ch.id } }" class="character">
              <TokenBadge :name="ch.name" allegiance="party" :icon-url="ch.tokenUrl ?? ''" :size="40" />
              <span class="who">{{ ch.name }}<template v-if="ch.mine"> (yours)</template></span>
              <span class="hint">Level {{ ch.level }} {{ ch.species }} {{ ch.class }} · {{ ch.ownerName }}</span>
              <span class="g-tag">{{ ch.hpCurrent }}/{{ ch.hpMax }} HP</span>
              <span v-if="ch.heroicInspiration" class="g-tag inspired" :data-testid="`inspired-${ch.name}`">Heroic Inspiration</span>
            </RouterLink>
          </li>
        </ul>
        <RouterLink :to="{ name: 'character-new', params: { id } }" class="build" data-testid="build-character">Build a character</RouterLink>
      </section>

      <CompanionsSection :campaign-id="id" :dm="isDM" :members="campaign.data.value.members" />

      <section class="g-card">
        <h2>Members</h2>
        <ul class="g-list" data-testid="member-list">
          <li v-for="m in campaign.data.value.members" :key="m.id" class="member">
            <span class="who">
              {{ m.displayName }}<template v-if="m.isMe"> (you)</template>
            </span>
            <span class="g-tag">{{ m.role === 'dm' ? 'DM' : 'Player' }}</span>
            <span v-if="isDM" class="actions">
              <GButton v-if="m.role === 'player'" @click="changeRole(m, 'dm')">Make co-DM</GButton>
              <GButton v-else @click="changeRole(m, 'player')">Make Player</GButton>
              <GButton v-if="!m.isMe" variant="danger" @click="removeMember(m)">Remove</GButton>
            </span>
          </li>
        </ul>
        <GButton v-if="!isDM" variant="danger" data-testid="leave" @click="removeMember(campaign.data.value.me)">Leave campaign</GButton>
      </section>

      <section v-if="isDM" class="g-card" data-testid="invites">
        <h2>Invite players</h2>
        <p>Anyone with the link joins as a Player. Links last a week.</p>
        <GButton variant="primary" data-testid="create-invite" @click="inviteLink()">Create invite link</GButton>
        <div v-if="newLink" class="link">
          <label class="g-field">
            <span>New link, shown once</span>
            <input :value="newLink" readonly data-testid="invite-link" />
          </label>
          <GButton @click="copy()">{{ copied ? 'Copied' : 'Copy' }}</GButton>
        </div>
        <ul class="g-list" data-testid="invite-list">
          <li v-for="inv in invites.data.value ?? []" :key="inv.id" class="member">
            <span class="who">By {{ inv.createdBy }}, until {{ expires(inv.expiresAt) }}</span>
            <GButton variant="danger" @click="revokeInvite(inv.id)">Revoke</GButton>
          </li>
        </ul>
      </section>
      <form v-if="isDM" class="g-card settings" data-testid="settings" @submit.prevent="saveSettings()">
        <h2>Table settings</h2>
        <label class="g-field">
          <span>Seconds to answer a reaction</span>
          <input v-model.number="timeout" type="number" min="3" max="120" data-testid="reaction-timeout" />
        </label>
        <label class="check">
          <input v-model="highGround" type="checkbox" data-testid="high-ground" />
          <span>High ground gives +2 to hit (optional rule)</span>
        </label>
        <label class="check">
          <input v-model="restSupplies" type="checkbox" data-testid="rest-supplies" />
          <span>A Long Rest costs each Character a day of Rations (optional rule)</span>
        </label>
        <label class="g-field">
          <span>Initiative</span>
          <select v-model="initiativeMode" data-testid="initiative-mode">
            <option value="individual">Each creature rolls</option>
            <option value="side">One roll per side</option>
          </select>
        </label>
        <label class="check">
          <input v-model="shareInitiative" type="checkbox" data-testid="share-initiative" />
          <span>Identical monsters share one roll</span>
        </label>
        <fieldset class="methods">
          <legend>New Characters set ability scores by</legend>
          <label class="check"><input v-model="creationMethods" type="checkbox" value="standard-array" data-testid="method-standard-array" /><span>Standard array</span></label>
          <label class="check"><input v-model="creationMethods" type="checkbox" value="point-buy" data-testid="method-point-buy" /><span>Point buy</span></label>
          <label class="check"><input v-model="creationMethods" type="checkbox" value="rolled" data-testid="method-rolled" /><span>Rolling 4d6, dropping the lowest</span></label>
        </fieldset>
        <label class="g-field">
          <span>New Characters start at level</span>
          <input v-model.number="startingLevel" type="number" min="1" max="20" data-testid="starting-level-input" />
        </label>
        <label class="check">
          <input v-model="holdLevelUps" type="checkbox" data-testid="hold-level-ups" />
          <span>Hold level-ups: long rests stop unlocking the next level, and you grant levels from each sheet</span>
        </label>
        <label class="check">
          <input v-model="noUndo" type="checkbox" data-testid="no-undo-setting" />
          <span>Play without undo: nothing is taken back, no Checkpoints, no rewind. It takes effect when the next Session starts</span>
        </label>
        <label class="check">
          <input v-model="showDcs" type="checkbox" data-testid="show-dcs-setting" />
          <span>Show the DC of a check on its Roll Card. Without it the DC is yours to know. It takes effect when the next Session starts</span>
        </label>
        <label class="g-field">
          <span>Exhaustion</span>
          <select v-model="exhaustion" data-testid="exhaustion-variant">
            <option value="srd-2024">SRD: −2 per level to D20 Tests, 5 feet of Speed, death at 6</option>
            <option value="gentle">Gentle: −1 per level, 5 feet of Speed, death at 10</option>
            <option value="grim">Grim: −2 per level, 10 feet of Speed, death at 4</option>
            <option value="off">Off: exhaustion does nothing</option>
          </select>
        </label>
        <GButton type="submit" :disabled="!creationMethods.length">Save settings</GButton>
        <p v-if="settings.isSuccess.value" role="status" data-testid="settings-saved">Saved.</p>
      </form>
    </template>
  </main>
</template>

<style scoped>
.methods {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  border: 0;
}
.back {
  color: var(--color-gold-high);
}
.sub {
  margin: 4px 0 0;
  font-family: var(--font-flavour);
  font-style: italic;
  color: var(--color-text-2);
}
h2 {
  margin: 0 0 10px;
  font-family: var(--font-display);
  font-size: 18px;
}
section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.member {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid var(--color-line);
}
.who {
  flex: 1 1 140px;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.link {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.prep {
  display: flex;
  flex-wrap: wrap;
  gap: 0 16px;
}
.prep a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--color-gold-high);
}
.character {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 10px;
  min-height: 44px;
  color: var(--color-text);
  text-decoration: none;
}
.hint {
  color: var(--color-text-2);
  font-size: 14px;
}
.build {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  min-height: 44px;
  padding: 0 16px;
  border: 1px solid var(--color-gold);
  border-radius: var(--radius-md);
  color: var(--color-gold-high);
  text-decoration: none;
}
.link .g-field {
  flex: 1 1 240px;
  min-width: 0;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.inspired {
  border-color: var(--color-gold-high);
  color: var(--color-gold-high);
}
</style>
