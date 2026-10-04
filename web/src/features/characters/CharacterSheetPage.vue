<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  deleteCharacterMutation,
  getCharacterOptions,
  getMyCharacterOptions,
  approveRetrainMutation,
  declineRetrainMutation,
  listCharactersOptions,
  listRetrainsOptions,
  listTracksOptions,
  passInspirationMutation,
  updateCharacterMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, TokenBadge } from '@/shared/ui'
import PortraitEditor from './PortraitEditor.vue'
import TokenEditor from './TokenEditor.vue'
import HitPointsPanel from './HitPointsPanel.vue'
import { titleCase } from '@/features/compendium/highlight'
import { abbrev, signed, type HitPointChange } from './build'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const path = computed(() => ({ path: { campaignId: String(route.params.id), characterId: String(route.params.characterId) } }))
const refresh = () => void client.invalidateQueries()
const sheet = useQuery(computed(() => ({ ...getCharacterOptions(path.value), retry: false })))
const s = computed(() => sheet.data.value)
const owned = useQuery(
  computed(() => ({
    ...getMyCharacterOptions({ path: { characterId: s.value?.characterId ?? '' } }),
    enabled: Boolean(s.value?.mine && s.value.characterId),
    retry: false,
  })),
)
const campaigns = computed(() => owned.data.value?.campaigns ?? [])
// Where this Character stands on the Campaign's Tracks: its own score, and the party's on a Track kept for the party.
const campaignTracks = useQuery(computed(() => ({ ...listTracksOptions({ path: { campaignId: path.value.path.campaignId } }), retry: false })))
const tracks = computed(() =>
  (campaignTracks.data.value?.tracks ?? []).flatMap((t) => {
    const mine = t.standings.find((st) => (t.scope === 'party' ? !st.characterId : st.characterId === path.value.path.characterId))
    return mine ? [`${t.name}${t.scope === 'party' ? ", the party's" : ''}: ${String(mine.value)} (${String(t.min)} to ${String(t.max)})`] : []
  }),
)
const update = useMutation(updateCharacterMutation())
const pass = useMutation(passInspirationMutation())
const party = useQuery(computed(() => ({ ...listCharactersOptions({ path: { campaignId: path.value.path.campaignId } }), enabled: Boolean(s.value?.heroicInspiration && s.value.mine) })))
const allies = computed(() => (party.data.value ?? []).filter((c) => c.id !== s.value?.id && !c.heroicInspiration))
const passTo = ref('')
const isDM = computed(() => Boolean(s.value && !s.value.mine && s.value.editable))
const retrains = useQuery(computed(() => ({ ...listRetrainsOptions(path.value), enabled: isDM.value, retry: false })))
const waiting = computed(() => (retrains.data.value ?? []).find((r) => r.status === 'pending'))
const approve = useMutation(approveRetrainMutation())
const decline = useMutation(declineRetrainMutation())
function decide(ok: boolean) {
  const r = waiting.value
  if (!r) return
  failed.value = false
  const opts = { path: { campaignId: path.value.path.campaignId, retrainId: r.id } }
  const done = { onSuccess: refresh, onError: () => (failed.value = true) }
  if (ok) approve.mutate(opts, done)
  else decline.mutate(opts, done)
}
const remove = useMutation(deleteCharacterMutation())
const failed = ref(false)

/** The part of the sheet a phone shows; a wide screen shows every part side by side. */
type Part = 'stats' | 'combat' | 'features' | 'gear'
const parts: { value: Part; label: string }[] = [
  { value: 'stats', label: 'Abilities' },
  { value: 'combat', label: 'Combat' },
  { value: 'features', label: 'Features' },
  { value: 'gear', label: 'Gear' },
]
const part = ref<Part>('combat')

function changeHp(body: HitPointChange) {
  failed.value = false
  update.mutate({ ...path.value, body }, { onSuccess: refresh, onError: () => (failed.value = true) })
}
function switchCampaign(ev: Event) {
  const entry = campaigns.value.find((c) => c.campaignId === (ev.target as HTMLSelectElement).value)
  if (entry) void router.push({ name: 'character', params: { id: entry.campaignId, characterId: entry.characterId } })
}
const classLine = computed(() => {
  const c = s.value?.classes ?? []
  if (c.length <= 1) return s.value?.class.name ?? ''
  return c.map((x) => `${x.name} ${String(x.level)}${x.subclass ? ` (${titleCase(x.subclass)})` : ''}`).join(' / ')
})
function inspire(inspired: boolean) {
  failed.value = false
  update.mutate({ ...path.value, body: { heroicInspiration: inspired } }, { onSuccess: refresh, onError: () => (failed.value = true) })
}
function passInspiration() {
  failed.value = false
  pass.mutate({ ...path.value, body: { to: passTo.value } }, { onSuccess: refresh, onError: () => (failed.value = true) })
}
function unlock() {
  failed.value = false
  update.mutate({ ...path.value, body: { levelUpReady: true } }, { onSuccess: refresh, onError: () => (failed.value = true) })
}
function destroy() {
  remove.mutate(path.value, {
    onSuccess: () => void router.push({ name: 'campaign', params: { id: path.value.path.campaignId } }),
    onError: () => (failed.value = true),
  })
}
const reach = (feet: number, range: number, long: number) => (range ? `${String(range)}/${String(long)} ft` : `${String(feet)} ft reach`)
</script>

<template>
  <main class="g-page sheet">
    <RouterLink :to="{ name: 'campaign', params: { id: path.path.campaignId } }" class="back">← Campaign</RouterLink>
    <p v-if="sheet.isPending.value">Unrolling the sheet…</p>
    <p v-else-if="sheet.isError.value" role="alert" class="g-alert" data-testid="sheet-missing">That character is not in this campaign.</p>
    <article v-else-if="s" data-testid="character-sheet">
      <header class="head">
        <img v-if="s.portraitUrl" :src="s.portraitUrl" :alt="`Portrait of ${s.name}`" class="portrait" data-testid="portrait" />
        <TokenBadge :name="s.name" allegiance="party" :icon-url="s.tokenUrl ?? ''" :size="56" data-testid="sheet-token" />
        <div class="title">
          <h1>{{ s.name }}</h1>
          <p class="sub" data-testid="sheet-classes">
            Level {{ s.level }} {{ s.species.name }} {{ classLine }} · {{ s.background.name }} · {{ s.ownerName }}
          </p>
          <p v-if="s.xp" class="sub" data-testid="sheet-xp">{{ s.xp }} XP</p>
          <p v-if="!s.editable" class="g-tag locked" data-testid="sheet-locked">Read only</p>
          <p v-if="s.editable" class="level-actions">
            <GButton v-if="s.levelUpReady" variant="primary" data-testid="level-up" @click="router.push({ name: 'level-up', params: { id: path.path.campaignId, characterId: path.path.characterId } })">
              Level up to {{ s.level + 1 }}
            </GButton>
            <GButton v-else-if="!s.mine && s.level < 20" data-testid="unlock-level" @click="unlock()">Grant level {{ s.level + 1 }}</GButton>
            <RouterLink class="spells-link" :to="{ name: 'character-spells', params: { id: path.path.campaignId, characterId: path.path.characterId } }" data-testid="open-spells">Spells</RouterLink>
            <RouterLink class="spells-link" :to="{ name: 'character-inventory', params: { id: path.path.campaignId, characterId: path.path.characterId } }" data-testid="open-inventory">Inventory</RouterLink>
            <RouterLink v-if="s.mine" class="spells-link" :to="{ name: 'character-retrain', params: { id: path.path.campaignId, characterId: path.path.characterId } }" data-testid="open-retrain">Retrain</RouterLink>
          </p>
        </div>
        <label v-if="campaigns.length > 1" class="switch">
          <span class="label">Campaign</span>
          <select :value="path.path.campaignId" data-testid="campaign-switch" @change="switchCampaign">
            <option v-for="c in campaigns" :key="c.campaignId" :value="c.campaignId">{{ c.campaignName }} · level {{ c.level }}</option>
          </select>
        </label>
      </header>

      <section class="vitals" aria-label="Vitals">
        <div class="stat shield"><span class="label">Armor Class</span><strong data-testid="ac">{{ s.armorClass }}</strong></div>
        <div class="stat"><span class="label">Initiative</span><strong>{{ signed(s.initiative) }}</strong></div>
        <div class="stat"><span class="label">Speed</span><strong>{{ s.speedFeet }} ft</strong></div>
        <div class="stat"><span class="label">Proficiency</span><strong>{{ signed(s.proficiencyBonus) }}</strong></div>
        <div class="stat"><span class="label">Passive Perception</span><strong>{{ s.passivePerception }}</strong></div>
      </section>
      <section v-if="waiting" class="g-card retrain-request" aria-label="Retrain request" data-testid="retrain-waiting">
        <p>
          {{ waiting.requestedBy }} asks to retrain {{ s.name }}<template v-if="waiting.reason">: {{ waiting.reason }}</template>.
          New build: {{ titleCase(waiting.proposed.species) }} {{ titleCase(waiting.proposed.background) }},
          {{ waiting.proposed.picks.map((p) => titleCase(p.value)).join(', ') || 'no level choices' }}.
        </p>
        <span class="decide">
          <GButton variant="primary" data-testid="approve-retrain" @click="decide(true)">Approve</GButton>
          <GButton data-testid="decline-retrain" @click="decide(false)">Decline</GButton>
        </span>
      </section>
      <section class="inspiration" :class="{ on: s.heroicInspiration }" aria-label="Heroic Inspiration" data-testid="inspiration">
        <span><strong class="small">Heroic Inspiration</strong> {{ s.heroicInspiration ? 'yours to spend on a reroll' : 'none' }}</span>
        <GButton v-if="s.editable && !s.mine" :data-testid="s.heroicInspiration ? 'take-inspiration' : 'grant-inspiration'" @click="inspire(!s.heroicInspiration)">
          {{ s.heroicInspiration ? 'Take it back' : 'Grant it' }}
        </GButton>
        <template v-if="s.mine && s.heroicInspiration && allies.length">
          <label class="pass">
            <span class="label">Pass it to</span>
            <select v-model="passTo" data-testid="pass-to">
              <option value="">Choose an ally</option>
              <option v-for="a in allies" :key="a.id" :value="a.id">{{ a.name }}</option>
            </select>
          </label>
          <GButton :disabled="!passTo || pass.isPending.value" data-testid="pass-inspiration" @click="passInspiration()">Pass</GButton>
        </template>
      </section>
      <HitPointsPanel :current="s.hpCurrent" :max="s.hpMax" :temp="s.tempHp ?? 0" :editable="s.editable" @change="changeHp" />
      <p v-if="failed" role="alert" class="g-alert">That change was not saved.</p>
      <ul v-if="s.warnings.length" class="warnings">
        <li v-for="w in s.warnings" :key="w">{{ w }}</li>
      </ul>

      <nav class="parts" aria-label="Sheet sections">
        <button v-for="p in parts" :key="p.value" type="button" :aria-pressed="part === p.value" :data-testid="`part-${p.value}`" @click="part = p.value">
          {{ p.label }}
        </button>
      </nav>

      <div class="columns">
        <div class="column" :class="{ shown: part === 'stats' }" data-part="stats">
          <section aria-label="Abilities" class="abilities" data-testid="abilities">
            <div v-for="a in s.abilities" :key="a.ability" class="ability">
              <span class="abbr">{{ abbrev[a.ability] }}</span>
              <strong>{{ signed(a.modifier) }}</strong>
              <span class="score">{{ a.score }}</span>
              <span class="save" :class="{ prof: a.saveProficient }">Save {{ signed(a.save) }}</span>
            </div>
          </section>
          <section class="g-card">
            <h2>Skills</h2>
            <ul class="skills" data-testid="skills">
              <li v-for="k in s.skills" :key="k.skill" :class="{ prof: k.proficient }">
                <span>
                  <span class="pip" :class="{ on: k.proficient, twice: k.expertise }" :aria-label="k.expertise ? 'Expertise' : k.proficient ? 'Proficient' : 'Untrained'" role="img" />
                  {{ titleCase(k.skill) }} <small>{{ abbrev[k.ability] }}</small>
                </span>
                <span>{{ signed(k.bonus) }}</span>
              </li>
            </ul>
          </section>
        </div>

        <div class="column" :class="{ shown: part === 'combat' }" data-part="combat">
          <section class="g-card">
            <h2>Attacks</h2>
            <p v-if="!s.attacks?.length" class="hint">No weapons carried.</p>
            <table v-else class="attacks" data-testid="attacks">
              <thead>
                <tr><th scope="col">Weapon</th><th scope="col">To hit</th><th scope="col">Damage</th></tr>
              </thead>
              <tbody>
                <tr v-for="a in s.attacks" :key="a.name">
                  <th scope="row">
                    {{ a.name }}
                    <small>{{ reach(a.reachFeet, a.rangeFeet, a.longRangeFeet) }}<template v-if="a.mastery"> · {{ titleCase(a.mastery) }} mastery</template></small>
                  </th>
                  <td>{{ signed(a.toHit) }}</td>
                  <td>{{ a.damage }} {{ a.damageType }}</td>
                </tr>
              </tbody>
            </table>
          </section>
          <section class="g-card">
            <h2>Resources</h2>
            <ul class="g-list" data-testid="resources">
              <li v-for="r in s.resources" :key="r.key">{{ r.label }}: {{ r.current }} / {{ r.max }}</li>
            </ul>
            <h2>Active effects</h2>
            <p v-if="s.effects.length === 0" class="hint">None right now.</p>
            <ul v-else class="g-list">
              <li v-for="e in s.effects" :key="e.name">{{ e.name }}: {{ e.detail }}</li>
            </ul>
            <template v-if="tracks.length > 0">
              <h2>Tracks</h2>
              <ul class="g-list" data-testid="sheet-tracks">
                <li v-for="line in tracks" :key="line">{{ line }}</li>
              </ul>
            </template>
          </section>
        </div>

        <div class="column" :class="{ shown: part === 'features' }" data-part="features">
          <section class="g-card">
            <h2>Features and traits</h2>
            <p v-if="!s.traits?.length" class="hint">Nothing yet.</p>
            <details v-for="t in s.traits" :key="`${t.source}-${t.name}`" class="trait" data-testid="trait">
              <summary>
                {{ t.name }} <small>{{ t.source === 'species' ? s.species.name : `${s.class.name} ${String(t.level)}` }}</small>
              </summary>
              <p>{{ t.description }}</p>
            </details>
          </section>
        </div>

        <div class="column" :class="{ shown: part === 'gear' }" data-part="gear">
          <section class="g-card">
            <h2>Equipment</h2>
            <p>{{ s.armor?.name ?? 'No armour' }}<template v-if="s.shield"> and a shield</template></p>
            <ul class="g-list">
              <li v-for="w in s.weapons" :key="w.slug">{{ w.name }}</li>
            </ul>
            <template v-if="s.proficiencies">
              <h2>Training</h2>
              <p data-testid="proficiencies">
                <strong class="small">Armor</strong> {{ s.proficiencies.armor.join(', ') || 'None' }}<br />
                <strong class="small">Weapons</strong> {{ s.proficiencies.weapons.join(', ') || 'None' }}
              </p>
            </template>
          </section>
          <PortraitEditor v-if="s.editable" :ids="path.path" @changed="refresh" />
          <TokenEditor
            v-if="s.editable"
            :name="s.name"
            :ids="path.path"
            :portrait-url="s.portraitUrl"
            :token-url="s.tokenUrl"
            @changed="refresh"
          />
          <GButton v-if="s.editable" variant="danger" data-testid="delete-character" @click="destroy()">Delete character</GButton>
        </div>
      </div>
    </article>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
article {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.title {
  flex: 1 1 200px;
}
h1 {
  margin: 0;
}
.portrait {
  width: 96px;
  height: 96px;
  object-fit: cover;
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-bronze);
}
.sub {
  margin: 4px 0 0;
  font-family: var(--font-flavour);
  font-style: italic;
  color: var(--color-text-2);
}
.locked {
  display: inline-block;
  margin-top: 6px;
}
.retrain-request {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-color: var(--color-gold-high);
}
.retrain-request p {
  margin: 0;
}
.decide {
  display: flex;
  gap: 8px;
}
.inspiration {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  padding: 10px 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.inspiration.on {
  border-color: var(--color-gold-high);
}
.pass {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.pass select {
  min-height: 44px;
  padding: 0 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font: inherit;
}
.level-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin: 8px 0 0;
}
.spells-link {
  color: var(--color-gold-high);
}
.switch {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.switch select {
  min-height: 44px;
  padding: 0 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font: inherit;
}
.vitals {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
  gap: 8px;
}
.stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 10px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  text-align: center;
}
.stat.shield {
  border-color: var(--color-bronze);
}
.label {
  font-size: 12px;
  color: var(--color-text-2);
}
strong {
  font-family: var(--font-display);
  font-size: 22px;
}
strong.small {
  font-size: 14px;
}
.warnings {
  margin: 0;
  color: var(--color-enemy-soft);
}
.parts {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--color-rule);
}
.parts button {
  flex: 1;
  min-height: 44px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--color-text-2);
  font: inherit;
  cursor: pointer;
}
.parts button[aria-pressed='true'] {
  color: var(--color-gold-high);
  border-bottom-color: var(--color-gold-high);
}
.columns {
  display: grid;
  gap: 14px;
}
.column {
  display: none;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}
.column.shown {
  display: flex;
}
@media (min-width: 960px) {
  .parts {
    display: none;
  }
  .columns {
    grid-template-columns: 1fr 1.2fr 1fr;
  }
  .column,
  .column[data-part='gear'] {
    display: flex;
  }
  .column[data-part='gear'] {
    grid-column: 1 / -1;
  }
}
.abilities {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.ability {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 4px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.abbr {
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.score,
.save {
  color: var(--color-text-2);
}
.save {
  font-size: 12px;
}
.save.prof,
.skills li.prof {
  color: var(--color-gold-high);
}
h2 {
  margin: 0 0 8px;
}
.skills {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: 2px;
}
.skills li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 28px;
}
.pip {
  display: inline-block;
  width: 10px;
  height: 10px;
  margin-right: 6px;
  border: 1px solid var(--color-text-2);
  border-radius: 50%;
}
.pip.on {
  background: var(--color-gold-high);
  border-color: var(--color-gold-high);
}
.pip.twice {
  box-shadow: 0 0 0 2px var(--color-surface), 0 0 0 3px var(--color-gold-high);
}
.attacks {
  width: 100%;
  border-collapse: collapse;
}
.attacks th,
.attacks td {
  padding: 6px 4px;
  border-bottom: 1px solid var(--color-rule);
  text-align: left;
  vertical-align: top;
}
.attacks thead th {
  font-size: 12px;
  font-weight: normal;
  color: var(--color-text-2);
}
.attacks small,
.trait small {
  display: block;
  color: var(--color-text-2);
}
.trait {
  border-bottom: 1px solid var(--color-rule);
  padding: 6px 0;
}
.trait summary {
  min-height: 32px;
  cursor: pointer;
}
.trait p {
  white-space: pre-line;
  color: var(--color-text-2);
}
.hint {
  color: var(--color-text-2);
}
</style>
