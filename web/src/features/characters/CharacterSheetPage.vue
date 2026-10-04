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
import CharacterTabs from './CharacterTabs.vue'
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
const campaignName = computed(() => campaigns.value.find((c) => c.campaignId === path.value.path.campaignId)?.campaignName ?? 'Campaign')
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
    <nav aria-label="Breadcrumb" class="g-crumbs">
      <RouterLink :to="{ name: 'campaign', params: { id: path.path.campaignId } }">{{ campaignName }}</RouterLink> <span aria-hidden="true">›</span>
      <template v-if="s"><span>{{ s.name }}</span> <span aria-hidden="true">›</span></template>
      <span aria-current="page">Sheet</span>
    </nav>
    <p v-if="sheet.isPending.value">Unrolling the sheet…</p>
    <p v-else-if="sheet.isError.value" role="alert" class="g-alert" data-testid="sheet-missing">That character is not in this campaign.</p>
    <article v-else-if="s" data-testid="character-sheet">
      <header class="head">
        <img v-if="s.portraitUrl" :src="s.portraitUrl" :alt="`Portrait of ${s.name}`" class="portrait" data-testid="portrait" />
        <TokenBadge :name="s.name" allegiance="party" :icon-url="s.tokenUrl ?? ''" :size="72" data-testid="sheet-token" />
        <div class="title">
          <h1>{{ s.name }}</h1>
          <p class="sub" data-testid="sheet-classes">
            Level {{ s.level }} {{ s.species.name }} {{ classLine }} · {{ s.background.name }} · {{ s.ownerName }}
          </p>
          <p v-if="s.xp" class="sub" data-testid="sheet-xp">{{ s.xp }} XP</p>
          <p v-if="!s.editable" class="g-tag locked" data-testid="sheet-locked">Read only</p>
        </div>
        <div class="acts">
          <label v-if="campaigns.length > 1" class="switch">
            <span class="label">Campaign</span>
            <select :value="path.path.campaignId" data-testid="campaign-switch" @change="switchCampaign">
              <option v-for="c in campaigns" :key="c.campaignId" :value="c.campaignId">{{ c.campaignName }} · level {{ c.level }}</option>
            </select>
          </label>
          <template v-if="s.editable">
            <GButton v-if="s.levelUpReady" variant="primary" data-testid="level-up" @click="router.push({ name: 'level-up', params: { id: path.path.campaignId, characterId: path.path.characterId } })">
              Level up to {{ s.level + 1 }}
            </GButton>
            <GButton v-else-if="!s.mine && s.level < 20" data-testid="unlock-level" @click="unlock()">Grant level {{ s.level + 1 }}</GButton>
          </template>
        </div>
      </header>

      <CharacterTabs :campaign-id="path.path.campaignId" :character-id="path.path.characterId" current="sheet" :others="s.editable" :retrain="s.mine" />

      <section class="vitals" aria-label="Vitals" data-testid="vitals">
        <div class="stat shield"><span class="label">Armour</span><strong data-testid="ac">{{ s.armorClass }}</strong></div>
        <div class="stat"><span class="label">Initiative</span><strong>{{ signed(s.initiative) }}</strong></div>
        <div class="stat"><span class="label">Speed</span><strong>{{ s.speedFeet }} ft</strong></div>
        <div class="stat"><span class="label">Proficiency</span><strong>{{ signed(s.proficiencyBonus) }}</strong></div>
        <div class="stat"><span class="label">Passive Perception</span><strong>{{ s.passivePerception }}</strong></div>
        <HitPointsPanel :current="s.hpCurrent" :max="s.hpMax" :temp="s.tempHp ?? 0" :editable="s.editable" @change="changeHp" />
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
            <h2>Abilities and saving throws</h2>
            <div v-for="a in s.abilities" :key="a.ability" class="ability">
              <span class="box"><strong>{{ signed(a.modifier) }}</strong><span class="score">{{ a.score }}</span></span>
              <span class="name"><b>{{ titleCase(a.ability) }}</b><span class="abbr">{{ abbrev[a.ability] }}</span></span>
              <span class="save" :class="{ prof: a.saveProficient }">
                <span class="pip" :class="{ on: a.saveProficient }" role="img" :aria-label="a.saveProficient ? 'Proficient' : 'Untrained'" />save <b>{{ signed(a.save) }}</b>
              </span>
            </div>
          </section>
          <section class="skills-part">
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
article {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 18px;
}
.title {
  display: flex;
  flex: 1 1 240px;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
h1 {
  margin: 0;
  font-size: 32px;
  overflow-wrap: anywhere;
}
.portrait {
  width: 72px;
  height: 72px;
  object-fit: cover;
  border: 1px solid var(--color-brass-line);
  border-radius: var(--radius-panel);
}
.sub {
  margin: 0;
  font-size: 15px;
  color: var(--color-text-2);
}
.locked {
  align-self: flex-start;
}
.acts {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 10px;
}
.switch,
.pass {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.switch select,
.pass select {
  min-height: var(--size-control);
  padding: 0 10px;
  border: 0;
  border-radius: var(--radius-control) var(--radius-control) 0 0;
  font: inherit;
  color: var(--color-text);
  background: var(--color-field);
  box-shadow: inset 0 -1px 0 var(--color-line);
}
/* The vitals are one inset band: the numbers a player reads off at a glance, then hit points. */
.vitals {
  display: flex;
  flex-wrap: wrap;
  align-items: stretch;
  gap: 4px;
  padding: 4px 8px;
  border-radius: var(--radius-panel);
  background: var(--color-field);
}
.stat {
  display: flex;
  flex: 0 1 auto;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  min-width: 84px;
  padding: 10px 8px;
  text-align: center;
}
.label {
  font-size: 12px;
  color: var(--color-text-3);
}
strong {
  font-family: var(--font-display);
  font-size: 26px;
  line-height: 1.1;
}
strong.small {
  font-family: var(--font-label);
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-text-3);
}
.vitals > :deep(.hp) {
  flex: 1 1 300px;
}
.inspiration {
  display: flex;
  flex: 0 1 auto;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  padding: 10px 8px;
  font-size: 14px;
  color: var(--color-text-2);
}
.inspiration > span:first-child {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.inspiration.on {
  color: var(--color-gold-high);
}
.retrain-request {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.retrain-request p {
  margin: 0;
}
.decide {
  display: flex;
  gap: 8px;
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
  font-family: var(--font-label);
  font-size: 15px;
  color: var(--color-text-2);
  background: none;
  cursor: pointer;
}
.parts button[aria-pressed='true'] {
  color: var(--color-gold-high);
  border-bottom-color: var(--color-gold);
}
.columns {
  display: grid;
  gap: 20px;
}
.column {
  display: none;
  flex-direction: column;
  gap: 24px;
  min-width: 0;
}
.column.shown {
  display: flex;
}
/* Wide: abilities, skills, and everything else, side by side. */
@media (min-width: 960px) {
  .parts {
    display: none;
  }
  .columns {
    grid-template-columns: 340px minmax(0, 1fr) minmax(0, 1.1fr);
    gap: 24px 40px;
    align-items: start;
  }
  .column[data-part='stats'] {
    display: contents;
  }
  .abilities {
    grid-row: 1 / span 3;
    grid-column: 1;
  }
  .skills-part {
    grid-row: 1 / span 3;
    grid-column: 2;
  }
  .column:not([data-part='stats']) {
    display: flex;
    grid-column: 3;
  }
}
h2 {
  margin: 0 0 8px;
}
.column > section.g-card {
  margin: 0;
  padding: 0;
  border: 0;
}
.ability {
  display: grid;
  grid-template-columns: 74px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-top: 1px solid var(--color-rule);
}
.box {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 4px 0;
  border-radius: var(--radius-control);
  background: var(--color-field);
}
.box strong {
  font-size: 22px;
}
.score {
  font-size: 12px;
  color: var(--color-text-2);
}
.name {
  display: flex;
  flex-direction: column;
}
.name b {
  font-size: 15px;
}
.abbr {
  font-size: 11px;
  letter-spacing: 0.08em;
  color: var(--color-text-3);
}
.save {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--color-text-2);
}
.save b {
  min-width: 24px;
  text-align: right;
  color: var(--color-text);
}
.skills {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;
}
.skills li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 5px 0;
  border-top: 1px solid var(--color-rule);
  font-size: 14px;
}
.skills li > span:first-child {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 8px;
}
.skills small {
  margin-left: auto;
  font-size: 11px;
  letter-spacing: 0.08em;
  color: var(--color-text-3);
}
.skills li > span:last-child {
  min-width: 34px;
  font-weight: 700;
  text-align: right;
}
.pip {
  display: inline-block;
  flex: none;
  width: 8px;
  height: 8px;
  border: 1px solid var(--color-edge);
  transform: rotate(45deg);
}
.pip.on {
  border-color: var(--color-gold-high);
  background: var(--color-gold-high);
}
.pip.twice {
  box-shadow: 5px -5px 0 -1px var(--color-gold-high);
  margin-right: 5px;
}
.attacks {
  width: 100%;
  border-collapse: collapse;
}
.attacks th,
.attacks td {
  padding: 8px 4px;
  border-top: 1px solid var(--color-rule);
  border-bottom: 0;
  text-align: left;
  vertical-align: top;
}
.attacks thead th {
  border-top: 0;
  font-size: 12px;
  font-weight: normal;
  color: var(--color-text-3);
}
.attacks small,
.trait small {
  display: block;
  font-size: 12px;
  color: var(--color-text-3);
}
.trait {
  padding: 6px 0;
  border-top: 1px solid var(--color-rule);
}
.trait summary {
  min-height: 32px;
  cursor: pointer;
}
.trait p {
  margin: 6px 0;
  font-family: var(--font-flavour);
  font-size: 16px;
  line-height: 1.5;
  white-space: pre-line;
  color: var(--color-text-2);
}
.hint {
  margin: 0;
  color: var(--color-text-3);
}
</style>
