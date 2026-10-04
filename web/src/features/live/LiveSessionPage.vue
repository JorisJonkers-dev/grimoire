<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { endSessionMutation, getCampaignOptions, getSessionLogOptions, listCharactersOptions, listCompanionsOptions, listFactionsOptions, listEncounterTablesOptions, listRuleVariantsOptions, listLootTablesOptions, listMapsOptions, listShopsOptions, listVehiclesOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { rollRest } from '@/infrastructure/api/sdk.gen'
import type { AmbientLight, LiveCombatant, LiveCombatantSetup, LiveSuggestion, LiveToken, MapObjectKind, TokenKind } from '@/infrastructure/api/types.gen'
import { useLiveSession } from '@/realtime/liveSession'
import type { Coord } from '@/shared/hex'
import HexGrid from '@/shared/map/HexGrid.vue'
import NotifyToggle from '@/shared/pwa/NotifyToggle.vue'
import { useWakeLock } from '@/shared/pwa/wakeLock'
import { GButton } from '@/shared/ui'
import { accessibility, buzz } from '@/shared/a11y/settings'
import { KEPT_LINES, rollLine, turnLine } from './announce'
import { BANNER_MS } from './motion'
import { DM_PAGES, PLAYER_PAGES, usePhoneShell } from './phoneShell'
import { after, board, describe, emanations, hexes, stacks, zoneHexes } from './board'
import { runByDM, suggested, taken } from './console'
import { cellsFor, key, layoutOf } from './geometry'
import DiceHost from '@/features/dice/DiceHost.vue'
import { throwDice } from '@/features/dice/stage'
import ControlSwitcher from './ControlSwitcher.vue'
import CreaturePanel from './CreaturePanel.vue'
import SpellList from './SpellList.vue'
import WalkPlan from './WalkPlan.vue'
import AreaPreviewCard from './AreaPreviewCard.vue'
import AttackPreview from './AttackPreview.vue'
import EffectsPanel from './EffectsPanel.vue'
import LegendPanel from './LegendPanel.vue'
import VisibilityPanel from './VisibilityPanel.vue'
import ObjectsPanel from './ObjectsPanel.vue'
import ActionLog from './ActionLog.vue'
import CheckpointPanel from './CheckpointPanel.vue'
import GroupsPanel from './GroupsPanel.vue'
import DyingPanel from './DyingPanel.vue'
import EncounterChecks from './EncounterChecks.vue'
import InventoryPanel from './InventoryPanel.vue'
import ReactionSettings from './ReactionSettings.vue'
import RestPanel from './RestPanel.vue'
import { tableResultLine } from './tableResult'
import Hotbar from './Hotbar.vue'
import RosterStrip from './RosterStrip.vue'
import EffectsCard from './EffectsCard.vue'
import LiveRoll from './LiveRoll.vue'
import MapBoard from './MapBoard.vue'
import ShopPanel from './ShopPanel.vue'
import ReactionPrompt from './ReactionPrompt.vue'
import StartCombat from './StartCombat.vue'
import TableRemote from './TableRemote.vue'
import TurnPanel from './TurnPanel.vue'
import WorldPanel from './WorldPanel.vue'
import ZonesPanel from './ZonesPanel.vue'

type Tool = 'tokens' | 'reveal' | 'conceal' | 'wall' | 'unwall' | 'light' | 'surface' | 'elevation' | 'zone' | 'object' | 'camera' | 'ping'

const route = useRoute()
const router = useRouter()
const campaignId = String(route.params.id)
const sessionId = String(route.params.sid)
const campaign = useQuery({ ...getCampaignOptions({ path: { campaignId } }), retry: false })
const isDM = computed(() => campaign.data.value?.myRole === 'dm')
const maps = useQuery(computed(() => ({ ...listMapsOptions({ path: { campaignId } }), enabled: isDM.value })))
const localMaps = computed(() => maps.data.value?.filter((m) => m.kind === 'local') ?? [])
const worldMaps = computed(() => maps.data.value?.filter((m) => m.kind === 'world') ?? [])
const scope = ref<'local' | 'world'>('local')
const lootTables = useQuery(computed(() => ({ ...listLootTablesOptions({ path: { campaignId } }), enabled: isDM.value, retry: false })))
const fightLoot = ref('')
const shops = useQuery(computed(() => ({ ...listShopsOptions({ path: { campaignId } }), enabled: isDM.value, retry: false })))
const encounterTables = useQuery(computed(() => ({ ...listEncounterTablesOptions({ path: { campaignId } }), enabled: isDM.value, retry: false })))
const characters = useQuery(computed(() => ({ ...listCharactersOptions({ path: { campaignId } }), enabled: isDM.value })))
const live = shallowRef<ReturnType<typeof useLiveSession> | null>(null)
const state = computed(() => live.value?.view)
// The screen stays on while the Session is live, so a phone on the table does not sleep mid-fight.
useWakeLock(() => state.value?.connection === 'open')
const view = computed(() => state.value?.view ?? null)

watch(
  () => campaign.data.value,
  (c) => {
    if (c && !live.value) live.value = useLiveSession(campaignId, sessionId, c.myRole === 'dm' ? 'dm' : 'party')
  },
  { immediate: true },
)
// A split party sends each screen to its own group's Session.
watch(
  () => state.value?.regroup,
  (to) => {
    if (to) void router.replace({ name: 'session', params: { id: campaignId, sid: to } })
  },
)
// The session opens in a watcher, outside setup, so the page closes it itself.
onBeforeUnmount(() => {
  clearTimeout(bannerTimer)
  live.value?.close()
})

const tool = ref<Tool>('tokens')
// From the phone's Table page the DM picks what the next tap on the map does, and goes to the map to do it.
function point(what: 'ping' | 'camera') {
  tool.value = what
  shell.page.value = 'map'
}
const pointing = computed(() => ({ ping: 'Tap the map to ping it.', camera: 'Tap the map to point the Table Display there.' } as Partial<Record<Tool, string>>)[tool.value])
const selected = ref<string | null>(null)
const label = ref('')
const kind = ref<TokenKind>('enemy')
const hidden = ref(false)
const darkvision = ref(0)
const brightFt = ref(20)
const dimFt = ref(40)
const mapChoice = ref('')
const controller = ref('')
const monster = ref('')
const knowsShield = ref(false)
const surfaceKind = ref('')
const surfaceRounds = ref(0)
const elevationFt = ref(10)
const character = ref('')
// The Companions of the Campaign that are not on the map yet, for the DM to put there.
const companions = useQuery(computed(() => ({ ...listCompanionsOptions({ path: { campaignId } }), enabled: isDM.value, retry: false })))
// The DM places a creature as one of a Faction, openly: its Standing then shapes social checks with it.
const factions = useQuery(computed(() => ({ ...listFactionsOptions({ path: { campaignId } }), retry: false })))
const faction = ref('')
// With slow natural healing a Long Rest spends Hit Dice too. The DM may switch it between rests, so it is asked again as each one starts.
const ruleVariants = useQuery({ ...listRuleVariantsOptions({ path: { campaignId } }), retry: false })
const slowHealing = computed(() => ruleVariants.data.value?.some((v) => v.slug === 'slow-natural-healing' && v.value === 'on') ?? false)
watch(() => view.value?.rest?.status, (status) => { if (status === 'resting') void ruleVariants.refetch() })
// How a creature takes to each Character an Influence check has moved it towards.
const attitudeNames = { hostile: 'Hostile', indifferent: 'Indifferent', friendly: 'Friendly' }
const characterName = (id: string) => characters.data.value?.find((c) => c.id === id)?.name ?? 'a Character'
// An Influence check is aimed at a creature: how its Faction regards whoever tries shapes the roll.
const swayed = ref('')
const swayable = computed(() => (view.value?.tokens ?? []).filter((t) => t.kind === 'npc' || t.kind === 'enemy'))
const factionName = (id: string) => factions.data.value?.find((f) => f.id === id)?.name ?? 'a Faction'

const companion = ref('')
const offMap = computed(() => (companions.data.value ?? []).filter((c) => !view.value?.tokens.some((t) => t.companionId === c.id)))
const players = computed(() => campaign.data.value?.members.filter((m) => m.role === 'player') ?? [])
const walkPath = computed(() => state.value?.path?.hexes ?? [])
const walkDanger = computed(() => state.value?.path?.threats.map((t) => ({ q: t.q, r: t.r })) ?? [])
const cells = computed(() =>
  board(state.value?.session?.gridRadius ?? 0, view.value?.tokens ?? [], selected.value, walkPath.value, {
    danger: walkDanger.value,
    captions: suggestions.value,
    surfaces: view.value?.surfaces,
    area: areaHexes.value,
    zone: zoneCells.value,
    reach: view.value?.sneak?.reach,
  }),
)
const chosen = computed(() => view.value?.tokens.find((t) => t.id === selected.value) ?? null)
const mine = computed(() => view.value?.tokens.filter((t) => t.controllerId && t.controllerId === campaign.data.value?.me.id) ?? [])
// A rider who controls its mount walks by walking the mount.
const steedOf = (t: LiveToken | null) => (t?.mountControlled ? (view.value?.tokens.find((o) => o.id === t.mountId) ?? t) : t)
const walker = computed(() => steedOf(isDM.value ? chosen.value : (mine.value.find((t) => t.id === selected.value) ?? mine.value[0] ?? null)))
// The hex the M key finds: where the creature I am playing stands, else the middle of the map.
const home = computed(() => {
  const t = walker.value ?? chosen.value
  return t ? `${String(t.q)},${String(t.r)}` : '0,0'
})
const combat = computed(() => view.value?.combat ?? null)
// The shared roster strip, and whose Effects the effects card shows.
const roster = computed(() => view.value?.roster ?? [])
const card = ref('')
const cardEntry = computed(() => roster.value.find((e) => e.tokenId === card.value))
// On a phone everyone swipes between pages over the map and pinches to zoom it; the DM's pages are a remote.
const pages = computed(() => (isDM.value ? DM_PAGES : PLAYER_PAGES))
const shell = usePhoneShell(() => pages.value)
// On a wide screen the drawer folds away to leave the map alone.
const folded = ref(false)
const actor = computed(() => combat.value?.combatants.find((c) => c.acting)?.label)
const chips = [
  { key: 'action', field: 'action', label: 'Action' },
  { key: 'bonus', field: 'bonusAction', label: 'Bonus' },
  { key: 'reaction', field: 'reaction', label: 'Reaction' },
] as const
// What a screen reader is told: each turn as it starts and each roll as it lands, the last few kept.
const lines = ref<{ no: number; text: string }[]>([])
let lineNo = 0
function say(text: string) {
  lines.value = [...lines.value, { no: ++lineNo, text }].slice(-KEPT_LINES)
}
// A player's roll, as it resolves anywhere at the table, is thrown on this screen's dice stage.
watch(
  () => state.value?.rolls,
  () => {
    const r = state.value?.roll
    if (!r) return
    throwDice(r, r.id)
    if (accessibility.announceRolls) say(rollLine(r))
  },
)
// "It's your turn" rises over a player's screen as their turn starts, then fades.
const banner = ref('')
let bannerTimer: ReturnType<typeof setTimeout> | undefined
watch(() => state.value?.turn?.n, () => {
  const starting = state.value?.turn?.tokenIds ?? []
  const acting = (view.value?.tokens ?? []).filter((t) => starting.includes(t.id)).map((t) => t.label)
  const names = mine.value.filter((t) => starting.includes(t.id)).map((t) => t.label)
  if (accessibility.announceTurns) say(turnLine(state.value?.turn?.round ?? 0, acting, names))
  if (names.length === 0) return
  clearTimeout(bannerTimer)
  banner.value = names.join(' and ')
  bannerTimer = setTimeout(() => { banner.value = '' }, BANNER_MS)
  buzz([120, 60, 120])
})
function dismissBanner() {
  clearTimeout(bannerTimer)
  banner.value = ''
}
const playable = (c: LiveCombatant) => isDM.value || (c.controllerId !== undefined && c.controllerId === campaign.data.value?.me.id)
// A mount is its rider's to play while the rider controls it.
const steered = (c: LiveCombatant) => mine.value.some((t) => t.mountControlled && t.mountId === c.tokenId)
const turns = computed(() => combat.value?.combatants.filter((c) => c.acting && (playable(c) || steered(c))) ?? [])
const toRoll = computed(() =>
  combat.value?.status === 'rolling'
    ? combat.value.combatants.filter((c) => c.initiative === undefined && (isDM.value ? !c.controllerId : playable(c)))
    : [],
)
const choosing = ref(false)
function startCombat(combatants: LiveCombatantSetup[]) {
  live.value?.send({ kind: 'start_combat', combatants })
  choosing.value = false
}
async function rollAll() {
  for (const c of toRoll.value) await rollRest({ path: { campaignId, rollId: c.rollId } }).catch(() => undefined)
}
const tokenById = (id: string) => view.value?.tokens.find((t) => t.id === id)
const pending = computed(() => combat.value?.attack ?? null)
const aiming = ref<{ tokenId: string; attackNo: number; offHand?: boolean; cleave?: boolean } | null>(null)
// grabbing is an Unarmed Strike waiting for its target: the next creature tapped is grappled or shoved.
const grabbing = ref<{ tokenId: string; option: string } | null>(null)
// The DM's console: the creatures in hand, one at a time following the turn, or several at once.
const creatures = computed(() => (isDM.value ? runByDM(view.value?.tokens ?? []) : []))
const inHand = ref<string[]>([])
const several = ref(false)
const actingCreatures = computed(() => turns.value.map((c) => c.tokenId).filter((id) => creatures.value.some((t) => t.id === id)))
watch(
  () => actingCreatures.value.join(),
  () => {
    if (!several.value && actingCreatures.value.length > 0) inHand.value = actingCreatures.value.slice(0, 1)
  },
  { immediate: true },
)
function setSeveral(on: boolean) {
  several.value = on
  if (!on) inHand.value = (actingCreatures.value.some((id) => inHand.value.includes(id)) ? actingCreatures.value : inHand.value).slice(0, 1)
}
const held = computed(() => creatures.value.filter((t) => inHand.value.includes(t.id)))
const combatantOf = (tokenId: string) => combat.value?.combatants.find((c) => c.tokenId === tokenId)
const sessionLog = useQuery(computed(() => ({ ...getSessionLogOptions({ path: { campaignId, sessionId }, query: { limit: 15 } }), enabled: isDM.value, retry: false })))
// The vehicles the DM may take the party aboard on the world map.
const fleet = useQuery(computed(() => ({ ...listVehiclesOptions({ path: { campaignId } }), enabled: isDM.value, retry: false })))
// Agent notes: what an agent did to a creature, newest first.
const notesFor = (tokenId: string) => (sessionLog.data.value ?? []).filter((a) => a.origin === 'mcp' && a.tokenId === tokenId)
const suggestions = computed(() =>
  Object.fromEntries(
    (isDM.value ? (combat.value?.combatants ?? []) : []).flatMap((c) => {
      const token = tokenById(c.tokenId)
      const said = token ? suggested(c, token, tokenById(c.suggestion?.targetId ?? '')?.label ?? 'its target') : undefined
      return said ? [[c.tokenId, said] as const] : []
    }),
  ),
)
const bars = computed(() =>
  turns.value.flatMap((c) => {
    const token = tokenById(c.tokenId)
    // A creature the DM runs shows its hotbar only while it is in hand.
    const shown = !creatures.value.some((t) => t.id === c.tokenId) || inHand.value.includes(c.tokenId)
    return token?.attacks?.length && shown ? [{ c, token }] : []
  }),
)
const blockedFor = (c: LiveCombatant) => (pending.value ? 'An attack is waiting on its roll.' : c.action ? '' : 'The action is used this turn.')
function arm(token: LiveToken, attackNo: number) {
  const same = aiming.value?.tokenId === token.id && aiming.value.attackNo === attackNo && !aiming.value.offHand
  aiming.value = same ? null : { tokenId: token.id, attackNo }
}
function armOffHand(token: LiveToken, attackNo: number) {
  aiming.value = { tokenId: token.id, attackNo, offHand: true }
}
function armCleave(token: LiveToken, attackNo: number) {
  aiming.value = { tokenId: token.id, attackNo, cleave: true }
}
const preview = computed(() => {
  const p = state.value?.preview
  return p && p.tokenId === aiming.value?.tokenId && p.attackNo === aiming.value.attackNo ? p : null
})
function confirmAttack(p: { tokenId: string; attackNo: number; targetId: string }) {
  live.value?.send({ kind: 'attack', tokenId: p.tokenId, attackNo: p.attackNo, targetId: p.targetId, ...(aiming.value?.offHand ? { offHand: true } : {}), ...(aiming.value?.cleave ? { cleave: true } : {}) })
  aiming.value = null
}
function useSuggestion(tokenId: string, s?: LiveSuggestion) {
  if (s?.attackNo !== undefined) live.value?.send({ kind: 'attack', tokenId, attackNo: s.attackNo, targetId: s.targetId })
}
// Whoever rolls a creature's saves sees their Roll Cards: its Controller, or the DM.
const mySaves = computed(() =>
  (view.value?.saves ?? []).filter((s) => {
    const owner = tokenById(s.tokenId)?.controllerId
    return isDM.value ? !owner : owner === campaign.data.value?.me.id
  }),
)
const areaAiming = ref<{ tokenId: string; effect: string; slot?: number; q?: number; r?: number } | null>(null)
function aimArea(token: LiveToken, effect: string, slot = 0) {
  aiming.value = null
  areaAiming.value = effect ? { tokenId: token.id, effect, ...(slot ? { slot } : {}) } : null
}
const areaPreview = computed(() => {
  const p = state.value?.areaPreview
  return p && p.tokenId === areaAiming.value?.tokenId && p.effect === areaAiming.value.effect ? p : null
})
const areaHexes = computed(() => [...(areaPreview.value?.hexes ?? view.value?.area?.hexes ?? []), ...emanations(view.value?.tokens ?? [])])
const teleporting = ref<string | null>(null)
const jumping = ref<string | null>(null)
// riding is the creature getting onto a mount, or off the one it rides; steer whether it will control it.
const riding = ref<{ tokenId: string; off: boolean } | null>(null)
const steer = ref(true)
// throwing is the thrower, then what it throws: a creature it grapples or an object next to it.
const throwing = ref<{ tokenId: string; targetId?: string; objectId?: string } | null>(null)
const surfaceKinds = computed(
  () => view.value?.surfaceKinds ?? ['fire', 'grease', 'water', 'ice', 'web', 'electrified'].map((kind) => ({ kind, name: (kind[0] ?? '').toUpperCase() + kind.slice(1) })),
)
const objectForm = ref<{ kind: MapObjectKind; name: string; secret: boolean; effect: string; radiusFt: number; detectDc: number; disarmDc: number; triggerFt: number; lockDc: number; key: string }>({
  kind: 'door', name: '', secret: false, effect: '', radiusFt: 0, detectDc: 0, disarmDc: 0, triggerFt: 0, lockDc: 0, key: '',
})
const summoning = ref<{ tokenId: string; effect: string } | null>(null)
// The creatures a Combatant summoned that wait for its command this round.
const awaitingOrders = (owner: string) =>
  (combat.value?.combatants ?? []).filter((c) => c.ownerId === owner && c.awaitingCommand).map((c) => ({ tokenId: c.tokenId, label: c.label }))
const zoneName = ref('')
const zoneRadius = ref(3)
const zoneDMOnly = ref(false)
const zoneCells = computed(() => {
  const m = view.value?.map
  const all = m ? cellsFor(layoutOf(m), m.width, m.height) : hexes(state.value?.session?.gridRadius ?? 0)
  return zoneHexes(view.value?.zones ?? [], all)
})
const checkRolls = computed(() => (isDM.value ? (view.value?.checks ?? []).filter((c) => c.status === 'pending' && c.rollId).map((c) => c.rollId ?? '') : []))
const myChecks = computed(() =>
  (view.value?.perception ?? []).filter((p) => {
    const owner = tokenById(p.tokenId)?.controllerId
    return isDM.value ? !owner : owner === campaign.data.value?.me.id
  }),
)
const names = computed(() => Object.fromEntries((view.value?.tokens ?? []).map((t) => [t.id, t.label])))
function castArea() {
  const a = areaAiming.value
  if (!a) return
  live.value?.send({ kind: 'cast_area', tokenId: a.tokenId, effect: a.effect, q: a.q ?? 0, r: a.r ?? 0, ...(a.slot ? { slot: a.slot } : {}) })
  areaAiming.value = null
}
// Whoever rolls a creature's dice sees the Roll Cards of an area spell: its damage and each save.
const rollsFor = (tokenId: string) => {
  const owner = tokenById(tokenId)?.controllerId
  return isDM.value ? !owner : owner === campaign.data.value?.me.id
}
const areaRolls = computed(() => {
  const a = view.value?.area
  if (!a) return []
  const damage = a.damageRollId && rollsFor(a.casterId) ? [a.damageRollId] : []
  return [...damage, ...a.saves.filter((s) => s.rollId && rollsFor(s.tokenId)).map((s) => s.rollId ?? '')]
})
const prompt = computed(() => combat.value?.prompt ?? null)
const answerable = computed(() => {
  const reactor = prompt.value ? tokenById(prompt.value.reactorId) : undefined
  return isDM.value || (reactor?.controllerId !== undefined && reactor.controllerId === campaign.data.value?.me.id)
})
// Whoever throws the attacker's dice sees the attack's Roll Card: its Controller, or the DM.
const attackRoll = computed(() => {
  const a = pending.value
  const attacker = a ? tokenById(a.attackerId) : undefined
  if (!a || !attacker) return null
  return (isDM.value ? !attacker.controllerId : attacker.controllerId === campaign.data.value?.me.id) ? a.rollId : null
})
const tokensAt = computed(() => stacks(view.value?.tokens ?? []))
const tokenAt = (c: Coord) => after(tokensAt.value, c, undefined)

// A tap on a hex only plans the walk there; nothing moves until the plan is confirmed.
function walkTo(c: Coord) {
  const t = walker.value
  if (t) live.value?.send({ kind: 'plan_walk', tokenId: t.id, q: c.q, r: c.r })
}
function confirmWalk() {
  const p = state.value?.path
  const end = p?.hexes.at(-1)
  if (p && end) live.value?.send({ kind: 'walk', tokenId: p.tokenId, q: end.q, r: end.r })
}
function tokenTool(c: Coord) {
  if (tokenAt(c)) {
    // A hex with a rider on its mount gives the rider, then the mount, then lets go.
    selected.value = after(tokensAt.value, c, selected.value)?.id ?? null
  } else if (chosen.value) {
    walkTo(c)
  } else if (companion.value) {
    live.value?.send({ kind: 'place_token', companionId: companion.value, q: c.q, r: c.r, hidden: hidden.value })
    companion.value = ''
  } else if (label.value.trim() || monster.value.trim() || character.value) {
    live.value?.send({
      kind: 'place_token', label: label.value.trim(), tokenKind: kind.value, q: c.q, r: c.r, hidden: hidden.value, darkvisionFt: darkvision.value,
      ...(controller.value ? { controllerId: controller.value } : {}),
      ...(character.value ? { characterId: character.value } : monster.value.trim() ? { monsterSlug: monster.value.trim() } : {}),
      ...(knowsShield.value ? { shield: true } : {}),
      ...(faction.value ? { factionId: faction.value } : {}),
    })
    label.value = ''
  }
}
function explore(c: Coord) {
  const there = tokenAt(c)
  if (there && mine.value.some((t) => t.id === there.id)) selected.value = there.id
  else walkTo(c)
}
function pick(c: Coord) {
  if (!live.value) return
  if (jumping.value) {
    live.value.send({ kind: 'jump', tokenId: jumping.value, q: c.q, r: c.r })
    jumping.value = null
    return
  }
  if (riding.value) {
    const { tokenId, off } = riding.value
    const mount = tokensAt.value.get(`${String(c.q)},${String(c.r)}`)?.find((t) => t.id !== tokenId)
    if (off) live.value.send({ kind: 'dismount', tokenId, q: c.q, r: c.r })
    else if (mount) live.value.send({ kind: 'mount', tokenId, targetId: mount.id, controlled: steer.value })
    else return
    riding.value = null
    return
  }
  if (throwing.value) {
    const t = throwing.value
    if (t.targetId || t.objectId) {
      live.value.send({ kind: 'throw', ...t, q: c.q, r: c.r })
      throwing.value = null
      return
    }
    const thing = tokenAt(c)
    const object = view.value?.objects?.find((o) => o.q === c.q && o.r === c.r)
    throwing.value = thing ? { ...t, targetId: thing.id } : object ? { ...t, objectId: object.id } : t
    return
  }
  if (summoning.value) {
    live.value.send({ kind: 'summon', ...summoning.value, q: c.q, r: c.r })
    summoning.value = null
    return
  }
  if (teleporting.value) {
    live.value.send({ kind: 'teleport', tokenId: teleporting.value, effect: 'misty-step', q: c.q, r: c.r })
    teleporting.value = null
    return
  }
  if (areaAiming.value) {
    areaAiming.value = { ...areaAiming.value, q: c.q, r: c.r }
    const { tokenId, effect, slot } = areaAiming.value
    live.value.send({ kind: 'preview_area', tokenId, effect, q: c.q, r: c.r, ...(slot ? { slot } : {}) })
    return
  }
  // Aimed at a hex whose rider the preview already shows, the next tap is at its mount.
  const target = aiming.value && preview.value ? (after(tokensAt.value, c, preview.value.targetId) ?? tokenAt(c)) : tokenAt(c)
  if (grabbing.value && target) {
    live.value.send({ kind: 'unarmed', tokenId: grabbing.value.tokenId, targetId: target.id, option: grabbing.value.option as 'grapple' })
    grabbing.value = null
    return
  }
  if (aiming.value && target) {
    live.value.send({ kind: 'preview_attack', ...aiming.value, targetId: target.id })
    return
  }
  if (!isDM.value) {
    explore(c)
    return
  }
  switch (tool.value) {
    case 'tokens':
      tokenTool(c)
      return
    case 'reveal':
    case 'conceal':
      live.value.send({ kind: 'reveal_hexes', hexes: [c], on: tool.value === 'reveal' })
      return
    case 'wall':
    case 'unwall':
      live.value.send({ kind: 'set_walls', hexes: [c], on: tool.value === 'wall' })
      return
    case 'surface':
      live.value.send({ kind: 'paint_surface', hexes: [c], ...(surfaceKind.value ? { surface: surfaceKind.value } : {}), ...(surfaceRounds.value ? { rounds: surfaceRounds.value } : {}) })
      return
    case 'elevation':
      live.value.send({ kind: 'set_elevation', hexes: [c], elevationFt: elevationFt.value })
      return
    case 'camera':
      live.value.send({ kind: 'table_camera', camera: 'free', q: c.q, r: c.r, zoomPct: view.value?.table?.zoomPct ?? 100 })
      return
    case 'ping':
      live.value.send({ kind: 'ping', q: c.q, r: c.r })
      return
    case 'object': {
      const f = objectForm.value
      live.value.send({
        kind: 'place_object', objectKind: f.kind, q: c.q, r: c.r, ...(f.name.trim() ? { objectName: f.name.trim() } : {}), ...(f.secret ? { secret: true } : {}),
        ...(f.effect.trim() ? { effect: f.effect.trim() } : {}), ...(f.radiusFt ? { radiusFt: f.radiusFt } : {}),
        ...(f.detectDc ? { detectDc: f.detectDc } : {}), ...(f.disarmDc ? { disarmDc: f.disarmDc } : {}), ...(f.triggerFt ? { triggerFt: f.triggerFt } : {}),
        ...(f.lockDc ? { lockDc: f.lockDc } : {}), ...(f.key.trim() ? { key: f.key.trim() } : {}),
      })
      return
    }
    case 'zone':
      if (zoneName.value.trim()) live.value.send({ kind: 'add_zone', label: zoneName.value.trim(), q: c.q, r: c.r, radiusHexes: zoneRadius.value, dmOnly: zoneDMOnly.value })
      return
    default: {
      const lit = view.value?.lights?.find((l) => key(l) === key(c))
      if (lit) live.value.send({ kind: 'remove_light', lightId: lit.id })
      else live.value.send({ kind: 'place_light', q: c.q, r: c.r, brightFt: brightFt.value, dimFt: dimFt.value })
    }
  }
}
function useMap() {
  live.value?.send(mapChoice.value ? { kind: 'set_map', mapId: mapChoice.value } : { kind: 'set_map' })
}
function setAmbient(a: AmbientLight) {
  live.value?.send({ kind: 'set_ambient', ambient: a })
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
const status = computed(() => ({ connecting: 'Connecting…', open: 'Live', reconnecting: 'Reconnecting…', ended: 'Session ended' })[state.value?.connection ?? 'connecting'])
</script>

<template>
  <main :class="['live', 'live--phone', { 'live--folded': folded }]">
    <p v-if="campaign.isError.value" role="alert" class="g-alert" data-testid="live-missing">This session is not available to you.</p>
    <template v-else-if="state">
      <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- a pinch is the touch shortcut; the zoom buttons do the same -->
      <div
        class="stage"
        data-testid="stage"
        @pointerdown="shell.pinchStart"
        @pointermove="shell.pinchMove"
        @pointerup="shell.pinchEnd"
        @pointercancel="shell.pinchEnd"
        @pointerleave="shell.pinchEnd"
      >
        <div class="scroller">
          <div class="zoomer" data-testid="zoomer" :style="{ width: `${String(Math.round(shell.zoom.value * 100))}%` }">
            <WorldPanel v-if="scope === 'world'" :world="view?.world" :dm="isDM" :maps="worldMaps" :local-maps="localMaps" :measure="state?.measure ?? null" :game-day="view?.gameDay ?? 0" :game-minute="view?.gameMinute ?? 0" :marching-order="view?.marchingOrder ?? []" :vehicles="fleet.data.value?.vehicles ?? []" @send="(cmd) => live?.send(cmd)" />
            <MapBoard v-else-if="view?.map" :map="view.map" :view="view" :dm="isDM" :selected="selected" :path="walkPath" :danger="walkDanger" :captions="suggestions" :area="areaHexes" :zone="zoneCells" :reach="view.sneak?.reach ?? []" :title="view.map.name" :home="home" @select="pick" />
            <HexGrid v-else :cells="cells" :title="`Session ${String(state.session?.number ?? '')} map`" :home="home" @select="pick" />
          </div>
        </div>
        <div class="zoom" role="group" aria-label="Map zoom">
          <button type="button" aria-label="Zoom out" :disabled="!shell.canZoomOut.value" data-testid="zoom-out" @click="shell.zoomOut">−</button>
          <button type="button" aria-label="Reset zoom" data-testid="zoom-reset" @click="shell.zoomReset">{{ Math.round(shell.zoom.value * 100) }}%</button>
          <button type="button" aria-label="Zoom in" :disabled="!shell.canZoomIn.value" data-testid="zoom-in" @click="shell.zoomIn">+</button>
        </div>
        <div id="quick-bar-slot" class="quick-slot" />
      </div>
      <div class="chrome">
        <header class="head" data-testid="live-bar">
          <RouterLink :to="{ name: 'home' }" class="brand">Grimoire<template v-if="isDM"> · DM</template></RouterLink>
          <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
          <div class="where"><span>{{ campaign.data.value?.name }} · </span><h1>Session {{ state.session?.number ?? '' }}</h1></div>
          <span class="g-tag" :class="`conn--${state.connection}`" data-testid="connection" role="status">{{ status }}</span>
          <strong v-if="combat?.status === 'active'" class="turn" data-testid="bar-turn">Round {{ combat.round }}<template v-if="actor"> · {{ actor }}</template></strong>
          <span class="acts">
            <RouterLink :to="{ name: 'table', params: { id: campaignId, sid: sessionId } }" class="table-link">Table display</RouterLink>
            <NotifyToggle />
            <button type="button" class="fold" :aria-expanded="!folded" aria-controls="live-drawer" data-testid="panels-toggle" @click="folded = !folded">Panels</button>
          </span>
        </header>
        <p v-if="state.rejection" role="alert" class="g-alert" data-testid="rejection">{{ state.rejection }}</p>
        <div class="top">
          <RosterStrip v-if="roster.length || combat" :roster="roster" :combat="combat ?? undefined" :tokens="view?.tokens ?? []" :reveal="state.reveal" @effects="(id) => (card = id)" />
          <EffectsCard v-if="cardEntry" :entry="cardEntry" @close="card = ''" />
          <WalkPlan v-if="state.path" :path="state.path" :mover="tokenById(state.path.tokenId)?.label ?? 'it'" @confirm="confirmWalk" @cancel="live?.dropPath()" />
          <p v-else-if="!isDM && walker" class="walk" data-testid="walker">Tap a hex to walk {{ walker.label }} there.</p>
          <fieldset class="scope" data-testid="scope">
            <legend class="sr-only">Which map</legend>
            <label v-for="s in (['local', 'world'] as const)" :key="s" :class="['scope-option', { on: scope === s }]">
              <input v-model="scope" type="radio" :value="s" :data-testid="`scope-${s}`" />
              <span>{{ s === 'local' ? 'Local' : 'World' }}</span>
            </label>
          </fieldset>
        </div>
      </div>
      <div class="sr-only" role="log" aria-live="polite" aria-label="What is happening" data-testid="announcer">
        <p v-for="l in lines" :key="l.no">{{ l.text }}</p>
      </div>
      <button v-if="banner" type="button" class="turn-banner" data-testid="turn-banner" @click="dismissBanner">
        <strong>It's your turn</strong> <span>{{ banner }}</span>
      </button>
      <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- a swipe is the touch shortcut; the page buttons do the same -->
      <div
        :class="['dock', `dock--page-${shell.page.value}`]"
        data-testid="dock"
        @pointerdown="shell.swipeStart"
        @pointerup="shell.swipeEnd"
      >
        <div class="tray" data-testid="tray">
          <p v-if="!isDM && turns.length > 0" data-page="always" class="banner" data-testid="your-turn">Your turn</p>
          <p v-if="view?.tableResult" data-page="always" role="status" class="walk" data-testid="table-result">{{ tableResultLine(view.tableResult) }}</p>
          <div v-if="view && !combat && (isDM || view.exploration)" data-page="actions" class="row" data-testid="exploration">
            <GButton v-if="isDM" :data-testid="view.exploration ? 'stop-turns' : 'start-turns'" @click="live?.send({ kind: 'explore', on: !view.exploration })">
              {{ view.exploration ? 'End exploration turns' : 'Explore in turns' }}
            </GButton>
            <template v-if="view.exploration">
              <span role="status" data-testid="exploration-turn">
                {{ names[view.exploration.turn] ?? 'Someone' }} explores · {{ view.exploration.leftFt }} ft left
              </span>
              <GButton
                v-if="isDM || tokenById(view.exploration.turn)?.controllerId === campaign.data.value?.me.id"
                data-testid="pass-turn"
                @click="live?.send({ kind: 'pass_turn' })"
              >
                Pass the turn
              </GButton>
            </template>
          </div>
          <div v-if="view && !combat" data-page="actions" class="row" data-testid="sneak">
            <GButton :data-testid="view.sneak ? 'stop-sneaking' : 'start-sneaking'" @click="live?.send({ kind: 'sneak', on: !view.sneak })">
              {{ view.sneak ? 'Stop sneaking' : 'Sneak' }}
            </GButton>
            <span v-if="view.sneak" role="status" data-testid="sneak-status">
              {{ view.sneak.waiting ? 'Sneaking: roll Stealth.' : 'Sneaking. Tinted hexes are watched.' }}
            </span>
          </div>
          <section v-if="toRoll.length > 0" data-page="always" class="rolls" aria-label="Initiative to roll">
            <GButton v-if="isDM && toRoll.length > 1" data-testid="roll-all" @click="rollAll()">Roll every initiative for me</GButton>
            <LiveRoll v-for="c in toRoll" :key="c.rollId" :campaign-id="campaignId" :roll-id="c.rollId" />
          </section>
          <p v-if="isDM && pointing" data-page="always" role="status" class="walk" data-testid="pointing">
            {{ pointing }}
            <GButton data-testid="stop-pointing" @click="tool = 'tokens'">Done</GButton>
          </p>
          <p v-if="summoning" data-page="always" role="status" class="walk" data-testid="summoning">Tap where they appear.</p>
          <p v-if="teleporting" data-page="always" role="status" class="walk" data-testid="teleporting">Tap a free hex within 30 feet.</p>
          <p v-if="jumping" data-page="always" role="status" class="walk" data-testid="jumping">Tap where to land.</p>
          <div v-if="riding" data-page="always" role="status" class="walk" data-testid="riding">
            <template v-if="riding.off">Tap a free hex next to your mount.</template>
            <template v-else>
              Tap the creature to ride.
              <label class="check"><input v-model="steer" type="checkbox" data-testid="ride-control" /><span>Control it</span></label>
            </template>
          </div>
          <p v-if="throwing" data-page="always" role="status" class="walk" data-testid="throwing">
            {{ throwing.targetId || throwing.objectId ? 'Tap where it lands.' : 'Tap the creature or object to throw.' }}
          </p>
          <p v-if="grabbing" data-page="always" role="status" class="walk" data-testid="grabbing">Tap the creature to grapple or shove.</p>
          <p v-if="areaAiming && !areaPreview" data-page="always" role="status" class="walk" data-testid="area-aiming">Tap where the spell goes.</p>
          <AreaPreviewCard v-if="areaPreview" data-page="always" :preview="areaPreview" :names="names" @confirm="castArea()" @cancel="areaAiming = null" />
          <LiveRoll v-for="id in areaRolls" :key="id" data-page="always" :campaign-id="campaignId" :roll-id="id" />
          <AttackPreview
            v-if="preview"
            data-page="always"
            :preview="preview"
            :target="tokenById(preview.targetId)?.label ?? 'the target'"
            @confirm="confirmAttack(preview)"
            @cancel="aiming = null"
          />
          <ReactionPrompt
            v-if="prompt"
            data-page="always"
            :prompt="prompt"
            :reactor="tokenById(prompt.reactorId)?.label ?? 'A creature'"
            :answerable="answerable"
            @answer="(use) => live?.send({ kind: 'react', use })"
          />
          <p v-if="pending" data-page="always" role="status" class="walk" data-testid="pending-attack">
            {{ pending.name }}{{ pending.critical ? ' (critical)' : '' }}: waiting for {{ { to_hit: 'the attack roll', reaction: 'a reaction', damage: 'the damage roll' }[pending.stage] }}.
          </p>
          <LiveRoll v-if="attackRoll" :key="attackRoll" data-page="always" :campaign-id="campaignId" :roll-id="attackRoll" />
          <LiveRoll v-for="s in mySaves" :key="s.rollId" data-page="always" :campaign-id="campaignId" :roll-id="s.rollId" />
          <LiveRoll v-for="p in myChecks" :key="p.rollId" data-page="always" :campaign-id="campaignId" :roll-id="p.rollId" />
          <LiveRoll v-for="id in checkRolls" :key="id" data-page="always" :campaign-id="campaignId" :roll-id="id" />
          <p v-if="view?.resolving" data-page="always" role="status" class="walk" data-testid="resolving">The DM is resolving an effect.</p>
          <ControlSwitcher
            v-if="isDM && creatures.length"
            data-page="actions"
            :creatures="creatures"
            :in-hand="inHand"
            :acting="actingCreatures"
            :several="several"
            @take="(id) => (inHand = taken(inHand, id, several))"
            @several="setSeveral"
            @tactics="(t) => t && held.forEach((x) => live?.send({ kind: 'set_tactics', tokenId: x.id, tactics: t }))"
            @hp="(d) => held.forEach((x) => live?.send({ kind: 'adjust_hp', tokenId: x.id, hpDelta: d }))"
          />
          <TurnPanel
            v-for="c in turns"
            :key="c.id"
            data-page="actions"
            :combatant="c"
            @spend="(r) => live?.send({ kind: 'spend', combatantId: c.id, resource: r })"
            @end="live?.send({ kind: 'end_turn', combatantId: c.id })"
          />
          <label v-if="bars.length > 0 && swayable.length > 0" class="g-field" data-page="actions">
            <span>Whom Influence is aimed at</span>
            <select v-model="swayed" data-testid="influence-target">
              <option value="">Nobody in particular</option>
              <option v-for="t in swayable" :key="t.id" :value="t.id">{{ t.label }}{{ t.factionId ? ` (${factionName(t.factionId)})` : '' }}</option>
            </select>
          </label>
          <Hotbar
            v-for="b in bars"
            :key="b.token.id"
            data-page="actions"
            :token="b.token"
            :armed="aiming?.tokenId === b.token.id ? aiming.attackNo : null"
            :blocked="blockedFor(b.c)"
            :suggestion="b.c.suggestion"
            :target="b.c.suggestion ? tokenById(b.c.suggestion.targetId)?.label : undefined"
            :tactics="b.c.tactics"
            :attacks-left="b.c.attacksLeft ?? 0"
            :off-hand="b.c.offHand ?? false"
            :interaction="b.c.interaction ?? false"
            :cleave="b.c.cleave ?? false"
            :summons="awaitingOrders(b.c.id)"
            :own="b.token.controllerId !== undefined && b.token.controllerId === campaign.data.value?.me.id"
            @arm="(n) => arm(b.token, n)"
            @use="useSuggestion(b.token.id, b.c.suggestion)"
            @tactics="(t) => live?.send({ kind: 'set_tactics', tokenId: b.token.id, tactics: t })"
            @area="(e, n) => aimArea(b.token, e, n)"
            @action="(a) => live?.send({ kind: 'take_action', tokenId: b.token.id, action: a as 'dash', ...(a === 'influence' && swayed ? { targetId: swayed } : {}) })"
            @ready="(n) => live?.send({ kind: 'take_action', tokenId: b.token.id, action: 'ready', trigger: 'enters_reach', attackNo: n })"
            @unarmed="(o) => (grabbing = { tokenId: b.token.id, option: o })"
            @off-hand="(n) => armOffHand(b.token, n)"
            @cleave="(n) => armCleave(b.token, n)"
            @interact="(d) => live?.send({ kind: 'interact', tokenId: b.token.id, detail: d })"
            @swap="live?.send({ kind: 'swap_weapons', tokenId: b.token.id })"
            @teleport="teleporting = b.token.id"
            @jump="jumping = b.token.id"
            @ride="riding = { tokenId: b.token.id, off: b.token.mountId !== undefined }"
            @throw="throwing = { tokenId: b.token.id }"
            @summon="(e) => (summoning = e ? { tokenId: b.token.id, effect: e } : null)"
            @command="(id) => live?.send({ kind: 'command', tokenId: b.token.id, targetId: id })"
          />
        </div>
        <div id="live-drawer" class="drawer" data-testid="drawer">
          <template v-if="isDM">
            <CreaturePanel
              v-for="t in held"
              :key="`creature-${t.id}`"
              data-page="actions"
              :token="t"
              :combatant="combatantOf(t.id)"
              :suggestion="suggestions[t.id]?.sentence"
              :notes="notesFor(t.id)"
              :no-undo="view?.noUndo ?? false"
              @tactics="(v) => live?.send({ kind: 'set_tactics', tokenId: t.id, tactics: v })"
              @use="useSuggestion(t.id, combatantOf(t.id)?.suggestion)"
              @undo="(seq) => live?.send({ kind: 'undo', seq })"
            />
          </template>
          <template v-if="!isDM">
            <SpellList
              v-for="b in bars"
              :key="`spells-${b.token.id}`"
              data-page="spells"
              :token="b.token"
              :blocked="blockedFor(b.c)"
              @area="(e, n) => (aimArea(b.token, e, n), (shell.page.value = 'map'))"
              @summon="(e) => ((summoning = { tokenId: b.token.id, effect: e }), (shell.page.value = 'map'))"
            />
          </template>
          <section v-if="view?.manual?.length" data-page="party" class="g-card manual" aria-label="Resolve by hand" data-testid="manual">
            <h2>Resolve by hand</h2>
            <ul class="g-list">
              <li v-for="m in view.manual" :key="m.id" class="row">
                <span>{{ m.text }}</span>
                <GButton :data-testid="`manual-done-${m.id}`" @click="live?.send({ kind: 'resolve_manual', manualId: m.id })">Done</GButton>
              </li>
            </ul>
          </section>
          <section v-if="isDM" v-show="scope === 'local'" data-page="tools" class="g-card controls" data-testid="dm-controls">
            <div class="row">
              <label class="g-field grow">
                <span>Map</span>
                <select v-model="mapChoice" data-testid="map-choice">
                  <option value="">No map (open grid)</option>
                  <option v-for="m in localMaps" :key="m.id" :value="m.id">{{ m.name }}</option>
                </select>
              </label>
              <GButton data-testid="use-map" @click="useMap()">Use map</GButton>
              <RouterLink :to="{ name: 'maps', params: { id: campaignId } }" class="manage">Manage maps</RouterLink>
            </div>
            <fieldset class="tools">
              <legend>Tap the map to</legend>
              <label v-for="t in (['tokens', 'reveal', 'conceal', 'wall', 'unwall', 'light', 'surface', 'elevation', 'zone', 'object', 'camera', 'ping'] as const)" :key="t" class="tool">
                <input v-model="tool" type="radio" :value="t" :data-testid="`tool-${t}`" />
                <span>{{ { tokens: 'Place or walk tokens', reveal: 'Reveal', conceal: 'Conceal', wall: 'Build walls', unwall: 'Clear walls', light: 'Place or remove light', surface: 'Paint surfaces', elevation: 'Raise or lower ground', zone: 'Draw an encounter zone', object: 'Place objects', camera: 'Point the table camera', ping: 'Ping the table' }[t] }}</span>
              </label>
            </fieldset>
            <div v-if="view?.map" class="row">
              <span>Ambient light</span>
              <GButton v-for="a in (['bright', 'dim', 'dark'] as const)" :key="a" :data-testid="`ambient-${a}`" @click="setAmbient(a)">
                {{ a }}{{ view.ambient === a ? ' ✓' : '' }}
              </GButton>
            </div>
            <div v-if="tool === 'surface'" class="row">
              <label class="g-field">
                <span>Surface</span>
                <select v-model="surfaceKind" data-testid="surface-kind">
                  <option value="">Clear</option>
                  <option v-for="k in surfaceKinds" :key="k.kind" :value="k.kind">{{ k.name }}</option>
                </select>
              </label>
              <label class="g-field"><span>Rounds</span><input v-model.number="surfaceRounds" type="number" min="0" max="100" data-testid="surface-rounds" /></label>
            </div>
            <div v-if="tool === 'zone'" class="row">
              <label class="g-field grow"><span>Zone name</span><input v-model="zoneName" maxlength="40" data-testid="zone-name" /></label>
              <label class="g-field"><span>Reach (hexes)</span><input v-model.number="zoneRadius" type="number" min="1" max="20" data-testid="zone-radius" /></label>
              <label class="check"><input v-model="zoneDMOnly" type="checkbox" data-testid="zone-dm-only" /><span>Only when I spring it</span></label>
            </div>
            <ZonesPanel v-if="view?.zones?.length" :zones="view.zones" :names="names" @send="(cmd) => live?.send(cmd)" />
            <div v-if="tool === 'object'" class="row">
              <label class="g-field">
                <span>Object</span>
                <select v-model="objectForm.kind" data-testid="object-kind">
                  <option v-for="k in ['door', 'lever', 'chest', 'barrel', 'curtain', 'destructible', 'trap'] as const" :key="k" :value="k">{{ k }}</option>
                </select>
              </label>
              <label class="g-field"><span>Name</span><input v-model="objectForm.name" maxlength="40" data-testid="object-name" /></label>
              <label class="g-field"><span>Triggers</span><input v-model="objectForm.effect" maxlength="80" placeholder="prone" data-testid="object-effect" /></label>
              <label class="g-field"><span>Reach (ft)</span><input v-model.number="objectForm.radiusFt" type="number" min="0" max="60" step="5" data-testid="object-radius" /></label>
              <label class="check"><input v-model="objectForm.secret" type="checkbox" data-testid="object-secret" /><span>Secret</span></label>
              <label class="g-field"><span>Spot DC</span><input v-model.number="objectForm.detectDc" type="number" min="0" max="40" data-testid="object-detect" /></label>
              <label class="g-field"><span>Disarm DC</span><input v-model.number="objectForm.disarmDc" type="number" min="0" max="40" data-testid="object-disarm" /></label>
              <label class="g-field"><span>Sets off within (ft)</span><input v-model.number="objectForm.triggerFt" type="number" min="0" max="60" step="5" data-testid="object-trigger" /></label>
              <label class="g-field"><span>Lock DC</span><input v-model.number="objectForm.lockDc" type="number" min="0" max="40" data-testid="object-lock" /></label>
              <label class="g-field"><span>Key</span><input v-model="objectForm.key" maxlength="80" placeholder="iron-key" data-testid="object-key" /></label>
            </div>
            <div v-if="tool === 'elevation'" class="row">
              <label class="g-field"><span>Height (ft)</span><input v-model.number="elevationFt" type="number" min="-100" max="100" step="5" data-testid="elevation-ft" /></label>
            </div>
            <div v-if="tool === 'light'" class="row">
              <label class="g-field"><span>Bright (ft)</span><input v-model.number="brightFt" type="number" min="0" max="600" /></label>
              <label class="g-field"><span>Dim (ft)</span><input v-model.number="dimFt" type="number" min="0" max="600" /></label>
            </div>
            <div v-if="tool === 'tokens'" class="row">
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
              <label class="g-field">
                <span>Controlled by</span>
                <select v-model="controller" data-testid="token-controller">
                  <option value="">Only the DM</option>
                  <option v-for="m in players" :key="m.id" :value="m.id">{{ m.displayName }}</option>
                </select>
              </label>
              <label class="g-field">
                <span>Monster</span>
                <input v-model="monster" placeholder="goblin" maxlength="80" data-testid="token-monster" />
              </label>
              <label class="g-field">
                <span>Character</span>
                <select v-model="character" data-testid="token-character">
                  <option value="">None</option>
                  <option v-for="ch in characters.data.value ?? []" :key="ch.id" :value="ch.id">{{ ch.name }}</option>
                </select>
              </label>
              <label v-if="isDM" class="g-field">
                <span>Faction</span>
                <select v-model="faction" data-testid="token-faction">
                  <option value="">No Faction</option>
                  <option v-for="f in factions.data.value ?? []" :key="f.id" :value="f.id">{{ f.name }}</option>
                </select>
              </label>
              <label v-if="isDM" class="g-field">
                <span>Companion</span>
                <select v-model="companion" data-testid="token-companion">
                  <option value="">None</option>
                  <option v-for="c in offMap" :key="c.id" :value="c.id">{{ c.name }}</option>
                </select>
              </label>
              <label class="g-field"><span>Darkvision (ft)</span><input v-model.number="darkvision" type="number" min="0" max="300" data-testid="token-darkvision" /></label>
              <label class="check"><input v-model="hidden" type="checkbox" data-testid="token-hidden" /><span>Hidden</span></label>
              <label class="check"><input v-model="knowsShield" type="checkbox" data-testid="token-shield" /><span>Knows Shield</span></label>
            </div>
            <LegendPanel v-if="chosen?.legend && isDM" :key="`legend-${chosen.id}`" :token="chosen" :legend="chosen.legend" @send="(c) => live?.send(c)" />
            <EffectsPanel
              v-if="chosen"
              :key="chosen.id"
              :token="chosen"
              :tokens="view?.tokens ?? []"
              :conditions="view?.conditions ?? []"
              @apply="(e) => live?.send({ kind: 'apply_effect', targetId: chosen!.id, ...e })"
              @end="(id, cured) => live?.send({ kind: 'end_effect', effectId: id, ...(cured ? { cured } : {}) })"
            />
            <VisibilityPanel
              v-if="chosen && isDM"
              :key="`vis-${chosen.id}`"
              :token="chosen"
              @set="(v) => live?.send({ kind: 'set_visibility', tokenId: chosen!.id, ...v })"
            />
            <div v-if="chosen" class="row" data-testid="selected-token">
              <span>{{ chosen.label }}{{ chosen.hidden ? ' (hidden)' : '' }}</span>
              <p v-if="chosen.factionId" class="hint" data-testid="token-faction-line">
                Of {{ factionName(chosen.factionId) }}.<template v-if="chosen.firstReaction"> First reaction: {{ chosen.firstReaction }}.</template>
              </p>
              <ul v-if="chosen.attitudes?.length" class="g-list" aria-label="Attitudes">
                <li v-for="a in chosen.attitudes" :key="a.characterId" data-testid="token-attitude">{{ attitudeNames[a.attitude] }} towards {{ characterName(a.characterId) }}</li>
              </ul>
              <label v-if="isDM && chosen.companionId" class="g-field">
                <span>Run by</span>
                <select
                  :value="chosen.controllerId ?? ''"
                  data-testid="token-hand"
                  @change="live?.send({ kind: 'assign_control', tokenId: chosen.id, ...(($event.target as HTMLSelectElement).value ? { controllerId: ($event.target as HTMLSelectElement).value } : {}) })"
                >
                  <option value="">The DM</option>
                  <option v-for="m in campaign.data.value?.members ?? []" :key="m.id" :value="m.id">{{ m.displayName }}</option>
                </select>
              </label>
              <GButton
                v-if="isDM && chosen.hp !== undefined"
                data-testid="ride-token"
                title="Put the creature on a willing creature next to it, or take it off the one it rides."
                @click="riding = { tokenId: chosen.id, off: chosen.mountId !== undefined }"
              >
                {{ chosen.mountId ? 'Dismount' : 'Mount' }}
              </GButton>
              <GButton data-testid="toggle-hidden" @click="toggleHidden()">{{ chosen.hidden ? 'Reveal' : 'Hide' }}</GButton>
              <GButton variant="danger" data-testid="remove-token" @click="remove()">Remove</GButton>
            </div>
            <div class="row">
              <GButton v-if="!view?.noUndo" data-testid="undo-damage" @click="live?.send({ kind: 'undo_damage' })">Undo last damage</GButton>
              <template v-if="combat">
                <label class="g-field">
                  <span>Loot when it ends</span>
                  <select v-model="fightLoot" data-testid="fight-loot">
                    <option value="">None</option>
                    <option v-for="t in lootTables.data.value ?? []" :key="t.id" :value="t.id">{{ t.name }}</option>
                  </select>
                </label>
                <GButton variant="danger" data-testid="end-combat" @click="live?.send(fightLoot ? { kind: 'end_combat', lootTableId: fightLoot } : { kind: 'end_combat' })">End combat</GButton>
              </template>
              <GButton v-else-if="!choosing" data-testid="choose-combatants" :disabled="!view?.tokens.length" @click="choosing = true">
                Start combat…
              </GButton>
            </div>
            <StartCombat v-if="choosing && !combat" :tokens="view?.tokens ?? []" @start="startCombat" />
          </section>
          <section v-if="isDM" data-page="table" class="g-card controls">
            <div class="row" role="group" aria-label="Point at the map">
              <GButton data-testid="remote-ping" @click="point('ping')">Ping the map</GButton>
              <GButton data-testid="remote-camera" @click="point('camera')">Point the Table Display</GButton>
            </div>
            <TableRemote
              :table="view?.table"
              :maps="worldMaps"
              @camera="(camera, zoomPct) => live?.send({ kind: 'table_camera', camera, zoomPct, q: view?.table?.q ?? 0, r: view?.table?.r ?? 0 })"
              @scene="(s) => live?.send({ kind: 'table_scene', ...s })"
              @blackout="(on) => live?.send({ kind: 'table_blackout', on })"
              @caption="(text) => live?.send({ kind: 'table_caption', caption: text })"
            />
            <GButton variant="danger" data-testid="end-session" @click="endSession()">End session</GButton>
          </section>
          <InventoryPanel
            v-if="view?.inventory?.length"
            data-page="party"
            :containers="view.inventory"
            :dm="isDM"
            :me="campaign.data.value?.me.id ?? ''"
            :loot-tables="lootTables.data.value ?? []"
            @send="(cmd) => live?.send(cmd)"
          />
          <ShopPanel
            v-if="isDM || view?.shop"
            data-page="party"
            :shop="view?.shop"
            :shops="shops.data.value ?? []"
            :containers="view?.inventory ?? []"
            :dm="isDM"
            :me="campaign.data.value?.me.id ?? ''"
            :campaign-id="campaignId"
            :game-day="view?.gameDay ?? 0"
            @send="(cmd) => live?.send(cmd)"
          />
          <DyingPanel v-if="view" data-page="character" :tokens="view.tokens" :dm="isDM" :helper="walker" @send="(cmd) => live?.send(cmd)" />
          <ReactionSettings
            v-if="walker?.attacks"
            data-page="character"
            :token="walker"
            @set="(kind, mode, condition) => live?.send({ kind: 'set_reaction', tokenId: walker?.id ?? '', reactionKind: kind, reactionMode: mode, condition })"
          />
          <ObjectsPanel v-if="view?.objects?.length" data-page="party" :objects="view.objects" :dm="isDM" :user="walker?.id" @send="(cmd) => live?.send(cmd)" />
          <RestPanel
            v-if="view"
            data-page="character"
            :rest="view.rest"
            :dm="isDM"
            :me="campaign.data.value?.me.id ?? ''"
            :tokens="view.tokens"
            :in-combat="Boolean(view.combat)"
            :hit-dice-in-long-rest="slowHealing"
            @send="(cmd) => live?.send(cmd)"
          />
          <EncounterChecks
            v-if="isDM || (view?.checks?.length ?? 0) > 0"
            data-page="party"
            :checks="view?.checks ?? []"
            :dm="isDM"
            :tables="encounterTables.data.value ?? []"
            @send="(cmd) => live?.send(cmd)"
          />
          <GroupsPanel
            v-if="isDM"
            data-page="tools"
            :campaign-id="campaignId"
            :groups="view?.groups ?? []"
            :tokens="view?.tokens ?? []"
            :maps="maps.data.value ?? []"
            @send="(cmd) => live?.send(cmd)"
          />
          <CheckpointPanel
            v-if="isDM"
            data-page="tools"
            :checkpoints="view?.checkpoints ?? []"
            :no-undo="view?.noUndo ?? false"
            :split="(view?.groups?.length ?? 0) > 1"
            @send="(cmd) => live?.send(cmd)"
          />
          <ActionLog v-if="isDM" data-page="party" :campaign-id="campaignId" :session-id="sessionId" :view="view" :no-undo="view?.noUndo ?? false" @undo="(seq) => live?.send({ kind: 'undo', seq })" />
          <ul data-page="party" class="g-list tokens" aria-label="Tokens in view" data-testid="tokens">
            <li v-for="t in view?.tokens ?? []" :key="t.id">{{ describe(t) }} · {{ t.kind }}</li>
          </ul>
        </div>
      </div>
      <div v-if="!isDM && turns.length" class="resources" role="group" aria-label="This turn" data-testid="resources">
        <template v-for="c in turns" :key="c.id">
          <span
            v-for="r in chips"
            :key="r.field"
            :class="['chip', { 'chip--spent': !c[r.field] }]"
            role="img"
            :aria-label="`${r.label}: ${c[r.field] ? 'available' : 'spent'}`"
            :data-testid="`chip-${r.key}`"
          >{{ r.label }}</span>
          <span class="chip chip--move">{{ c.movementFt }} / {{ c.speedFt }} ft</span>
        </template>
      </div>
      <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- a swipe is the touch shortcut; the buttons turn the pages too -->
      <nav class="pagebar" aria-label="Live play pages" data-testid="phone-pages" @pointerdown="shell.swipeStart" @pointerup="shell.swipeEnd">
        <button
          v-for="p in pages"
          :key="p.key"
          type="button"
          :class="['page', { 'page--on': shell.page.value === p.key }]"
          :aria-current="shell.page.value === p.key ? 'page' : undefined"
          :data-testid="`page-${p.key}`"
          :data-page-key="p.key"
          @click="shell.page.value = p.key"
        >
          {{ p.label }}
        </button>
      </nav>
    </template>
    <p v-else>Opening the session…</p>
    <DiceHost />
  </main>
</template>

<style scoped>
/* The map is the page: it fills the screen, and everything else floats over it on smoked glass. */
.live {
  --glass: rgb(14 11 9 / 58%);
  --glass-deep: rgb(14 11 9 / 78%);
  --glass-edge: rgb(217 190 126 / 16%);
  --glass-line: rgb(217 190 126 / 28%);
  --aside: 408px;
  position: relative;
  height: 100dvh;
  overflow: hidden;
  background: #0b0908;
}
.live--folded {
  --aside: 14px;
}
.live > p {
  margin: 0;
  padding: 24px;
}
.stage {
  position: absolute;
  inset: 0;
}
/* Padding lets every hex scroll out from under the bar, the roster, the tray and the drawer. */
.scroller {
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  padding: 172px var(--aside) 140px 14px;
  overflow: auto;
}
.zoomer {
  min-width: 50%;
  margin: 0 auto;
}
.zoomer :deep(.hex-scroll),
.zoomer :deep(.board-scroll) {
  width: fit-content;
  margin: 0 auto;
}
.zoom {
  position: absolute;
  bottom: 14px;
  left: 14px;
  z-index: 2;
  display: flex;
  flex-direction: column-reverse;
  gap: 4px;
}
.zoom button,
.fold,
.table-link,
.head :deep(button) {
  min-height: 30px;
  padding: 0 12px;
  border: 1px solid var(--glass-line);
  border-radius: var(--radius-control);
  font-family: var(--font-label);
  font-size: 13px;
  color: var(--color-text);
  background: rgb(33 26 20 / 70%);
  white-space: nowrap;
  cursor: pointer;
}
.zoom button {
  min-width: 44px;
  min-height: 36px;
  padding: 0 6px;
  backdrop-filter: blur(10px) saturate(1.15);
}
.zoom button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
.chrome {
  position: absolute;
  top: 14px;
  right: 14px;
  left: 14px;
  z-index: 3;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.chrome > * {
  pointer-events: auto;
}
.head,
.tray,
.drawer {
  border: 1px solid var(--glass-edge);
  border-radius: var(--radius-panel);
  background: var(--glass);
  backdrop-filter: blur(10px) saturate(1.15);
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 40px;
  padding: 0 6px 0 12px;
  font-size: 13px;
}
.brand {
  font-family: var(--font-display);
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  text-decoration: none;
  color: var(--color-bar-brand);
  white-space: nowrap;
}
.back {
  color: var(--color-gold-high);
  white-space: nowrap;
}
.where {
  overflow: hidden;
  font-size: 12px;
  color: var(--color-text-2);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.where h1 {
  display: inline;
  margin: 0;
  font: inherit;
  letter-spacing: 0;
  color: inherit;
}
.turn {
  color: var(--color-gold-high);
  white-space: nowrap;
}
.acts {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}
.table-link {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
}
.fold[aria-expanded='true'] {
  border-color: var(--color-brass-edge);
  color: var(--color-brass-text);
  background: var(--color-brass);
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
.chrome > .g-alert {
  align-self: center;
  margin: 0;
  background: var(--glass-deep);
}
/* The roster, a walk waiting for its yes, and the choice of map hang under the bar, clear of the drawer. */
.top {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-right: calc(var(--aside) - 14px);
  pointer-events: none;
}
.top > * {
  pointer-events: auto;
}
.top > :deep(.plan),
.top > :deep(.card),
.walk {
  margin: 0;
  padding: 6px 10px;
  border: 1px solid var(--glass-edge);
  border-radius: var(--radius-panel);
  background: var(--glass-deep);
  backdrop-filter: blur(12px);
}
.walk {
  color: var(--color-gold-high);
}
.scope {
  position: absolute;
  top: 0;
  left: 0;
  display: inline-flex;
  margin: 0;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--glass-edge);
  border-radius: var(--radius-panel);
  font-family: var(--font-label);
  font-size: 13px;
  background: var(--glass);
  backdrop-filter: blur(10px) saturate(1.15);
}
.scope-option {
  display: flex;
  align-items: center;
  min-height: 30px;
  padding: 0 12px;
  color: var(--color-text-2);
  cursor: pointer;
}
.scope-option input {
  position: absolute;
  opacity: 0;
}
.scope-option.on {
  color: var(--color-gold-high);
  background: rgb(217 190 126 / 16%);
}
.scope-option:has(input:focus-visible) {
  outline: 2px solid var(--color-gold-high);
  outline-offset: -2px;
}
/* The dock is two things on a wide screen: the turn in a tray at the foot of the map, and a drawer of
   everything else down the right-hand side. */
.dock {
  display: contents;
}
.tray {
  position: absolute;
  right: var(--aside);
  bottom: 14px;
  left: 14px;
  z-index: 2;
  display: flex;
  flex-direction: column;
  gap: 7px;
  width: fit-content;
  max-width: min(860px, calc(100% - var(--aside) - 142px));
  max-height: 46dvh;
  margin: 0 auto;
  padding: 8px 12px;
  overflow-y: auto;
}
.tray:not(:has(> *)) {
  display: none;
}
.tray > :deep(.g-card) {
  margin: 0;
  padding: 0;
  border: 0;
}
.tray > :deep(* + .g-card),
.tray > :deep(.g-card + *) {
  padding-top: 7px;
  border-top: 1px solid var(--glass-edge);
}
.tray :deep(h2) {
  margin: 0;
  font-family: var(--font-label);
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
/* In the tray a field is one short line: its name, then the choice. */
.tray :deep(.g-field) {
  flex-direction: row;
  align-items: center;
  gap: 8px;
}
.tray :deep(.g-field > span:first-child) {
  position: static;
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
  white-space: nowrap;
}
.tray :deep(.g-field select),
.tray :deep(.g-field input:not([type='checkbox'], [type='radio'])) {
  width: auto;
  min-width: 0;
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid var(--glass-line);
  border-radius: var(--radius-control);
  font-size: 13px;
  background: rgb(33 26 20 / 70%);
  box-shadow: none;
}
.drawer {
  position: absolute;
  top: 62px;
  right: 14px;
  bottom: 14px;
  z-index: 2;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  width: 380px;
  padding: 0 14px 14px;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--color-edge) transparent;
}
.live--folded .drawer {
  display: none;
}
.drawer > :deep(*) {
  margin: 0;
  padding: 12px 0;
  border: 0;
  border-top: 1px solid var(--glass-edge);
  border-radius: 0;
  background: none;
}
.drawer > :deep(:first-child) {
  border-top: 0;
}
.drawer :deep(.g-button) {
  min-height: 34px;
  padding: 0 12px;
  font-size: 14px;
}
.drawer :deep(.g-field) {
  flex: 1 1 130px;
  min-width: 0;
}
.drawer :deep(.g-field input:not([type='range'], [type='file'], [type='checkbox'], [type='radio'])),
.drawer :deep(.g-field select) {
  min-height: 48px;
  padding: 20px 10px 4px;
  font-size: 15px;
}
.drawer :deep(.g-field > span:first-child) {
  top: 5px;
  left: 10px;
  right: 6px;
  overflow: hidden;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.drawer :deep(h2) {
  margin: 0 0 8px;
  font-family: var(--font-label);
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
.pagebar,
.resources {
  display: none;
}
@media (min-width: 900px) and (max-width: 1199px) {
  .live:not(.live--folded) {
    --aside: 368px;
  }
  .drawer {
    width: 340px;
  }
  .where span {
    display: none;
  }
}
@media (pointer: coarse) {
  .zoom button,
  .fold,
  .table-link,
  .head :deep(button),
  .scope-option,
  .tool,
  .tray :deep(.g-field select),
  .tray :deep(.g-field input:not([type='checkbox'], [type='radio'])),
  .drawer :deep(.g-button) {
    min-height: 44px;
  }
}
/* A phone: the dock is a sheet over the foot of the map, attached to a bar of pages that runs edge to
   edge, and the turn's resources float above the bar. */
@media (max-width: 899px) {
  .live {
    --aside: 8px;
  }
  .scroller {
    padding: 204px 8px calc(170px + env(safe-area-inset-bottom));
    touch-action: pan-x pan-y;
  }
  .chrome {
    top: 8px;
    right: 8px;
    left: 8px;
  }
  .head {
    flex-wrap: wrap;
    gap: 4px 10px;
    padding: 4px 6px 4px 10px;
  }
  .brand,
  .where span,
  .fold {
    display: none;
  }
  .top {
    margin-right: 0;
  }
  .scope {
    position: static;
    align-self: flex-start;
  }
  /* The sheet rises from the foot of the screen, so the zoom keeps to the side, above it. */
  .zoom {
    top: 212px;
    right: 8px;
    bottom: auto;
    left: auto;
  }
  .quick-slot {
    position: absolute;
    right: 8px;
    bottom: calc(104px + env(safe-area-inset-bottom));
    left: 64px;
    z-index: 2;
  }
  .dock {
    position: fixed;
    right: 0;
    bottom: calc(96px + env(safe-area-inset-bottom));
    left: 0;
    z-index: 3;
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-height: 55dvh;
    padding: 10px 12px;
    overflow-y: auto;
    border-top: 1px solid rgb(217 190 126 / 20%);
    border-radius: 10px 10px 0 0;
    background: rgb(14 11 9 / 82%);
    backdrop-filter: blur(12px);
    /* Sideways movement is ours, to turn the page; up and down still scrolls the sheet. */
    touch-action: pan-y;
  }
  /* On the Map page the sheet is only what needs an answer: the map shows, and takes taps, around it. */
  .dock--page-map {
    border-top: 0;
    background: transparent;
    backdrop-filter: none;
    pointer-events: none;
  }
  .dock--page-map > * > * {
    padding: 6px 10px;
    border: 1px solid var(--glass-edge);
    border-radius: var(--radius-panel);
    background: var(--glass-deep);
    backdrop-filter: blur(12px);
    pointer-events: auto;
  }
  /* Neither part has a box here, but each must still pass the sheet's touch rule on to what it holds. */
  .tray,
  .drawer {
    display: contents;
    overflow: visible;
    touch-action: pan-y;
  }
  .live--folded .drawer {
    display: contents;
  }
  .dock--page-map > * > :not([data-page~='always']),
  .dock--page-actions > * > :not([data-page~='actions'], [data-page~='always']),
  .dock--page-spells > * > :not([data-page~='spells'], [data-page~='always']),
  .dock--page-character > * > :not([data-page~='character'], [data-page~='always']),
  .dock--page-party > * > :not([data-page~='party'], [data-page~='always']),
  .dock--page-table > * > :not([data-page~='table'], [data-page~='always']),
  .dock--page-tools > * > :not([data-page~='tools'], [data-page~='always']) {
    display: none;
  }
  .resources {
    position: fixed;
    right: 0;
    bottom: calc(60px + env(safe-area-inset-bottom));
    left: 0;
    z-index: 4;
    display: flex;
    justify-content: center;
    gap: 6px;
    padding: 6px 8px;
    background: rgb(14 11 9 / 82%);
    backdrop-filter: blur(12px);
  }
  .pagebar {
    position: fixed;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 4;
    display: grid;
    grid-auto-columns: 1fr;
    grid-auto-flow: column;
    padding-bottom: env(safe-area-inset-bottom);
    border-top: 1px solid var(--color-rule);
    background: var(--color-bar);
    touch-action: pan-y;
  }
}
.chip {
  padding: 2px 8px;
  border: 1px solid var(--color-success);
  border-radius: var(--radius-chip);
  font-size: 13px;
}
.chip--spent {
  border-color: var(--color-line);
  color: var(--color-text-2);
  text-decoration: line-through;
}
.chip--move {
  border-color: var(--color-line);
}
.page {
  min-height: 60px;
  border: 0;
  font-family: var(--font-label);
  font-size: 13px;
  color: var(--color-bar-text);
  background: transparent;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}
.page--on {
  color: var(--color-gold-high);
  box-shadow: inset 0 2px 0 var(--color-gold-high);
}
.turn-banner {
  position: fixed;
  top: 46%;
  left: 50%;
  z-index: 5;
  display: grid;
  justify-items: center;
  gap: 9px;
  padding: 0;
  border: 0;
  font-family: var(--font-label);
  font-size: 15px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-text);
  text-shadow: 0 1px 3px #000, 0 0 2px #000;
  background: none;
  transform: translate(-50%, -50%);
  animation: banner 2800ms ease forwards;
  cursor: pointer;
}
.turn-banner strong {
  font-family: var(--font-display);
  font-size: clamp(34px, 6vw, 72px);
  font-weight: 900;
  line-height: 1;
  letter-spacing: 0.02em;
  text-transform: none;
  color: var(--color-gold-high);
  text-shadow: 0 3px 0 var(--color-bar), 0 8px 22px rgb(0 0 0 / 90%);
}
/* The banner grows in and shrinks out rather than fading: half-faded text would be too faint to read. */
@keyframes banner {
  0% {
    transform: translate(-50%, -50%) scale(0);
  }
  12%,
  80% {
    transform: translate(-50%, -50%) scale(1);
  }
  100% {
    transform: translate(-50%, -50%) scale(0);
  }
}
@media (prefers-reduced-motion: reduce) {
  .turn-banner {
    animation: none;
  }
}
.controls {
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
.grow {
  flex: 1 1 160px;
}
.manage {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--color-gold-high);
}
.tools {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: 0;
  padding: 0;
  border: 0;
}
.tools legend {
  margin-bottom: 4px;
  font-family: var(--font-label);
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
/* What a tap on the map does is one chip of a set. */
.tool {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid rgb(217 190 126 / 22%);
  border-radius: var(--radius-chip);
  font-size: 13px;
  color: var(--color-text-2);
  cursor: pointer;
}
.tool input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  opacity: 0;
  cursor: pointer;
}
.tool:has(input:checked) {
  border-color: var(--color-brass-edge);
  color: var(--color-gold-high);
  background: rgb(217 190 126 / 14%);
}
.tool:has(input:focus-visible) {
  outline: 2px solid var(--color-gold-high);
  outline-offset: 2px;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.banner {
  margin: 0;
  font-family: var(--font-display);
  font-size: 18px;
  color: var(--color-gold-high);
}
.rolls {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.tray .walk,
.drawer .walk {
  padding: 0;
  border: 0;
  background: none;
  backdrop-filter: none;
}
.tokens {
  font-size: 14px;
  color: var(--color-text-2);
}
</style>
