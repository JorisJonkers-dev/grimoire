"""Cut the automation backlog: engine tickets and content batches with their blocking edges.

Reads units.json (every entry with its merged gaps, pattern, weight) and writes plan.json.
"""
import collections, json, math, re, sys
sys.path.insert(0, '.')
import engine_gaps as eg

SIZE = 20
WAVE = dict(eg.WAVE_OF)
WAVE.update({'monster_link': 0, 'environment': 3, 'sequence': 1, 'roll_rule': 1})
units = json.load(open('units.json'))
for u in units:
    u['wave'] = max([WAVE[g] for g in u['gaps']], default=0)

def T(key, title, body, wave, blocked=()):
    return dict(key=key, title=title, body=body, wave=wave, blocked=list(blocked), kind='engine')

engine = [
    T('glossary', 'Glossary: Activation, Note, Part, Resource, Add-on, Recharge, Variant, Scaling and Manual Effect',
      'Fold the glossary wording the map\'s tickets proposed into `CONTEXT.md`, and rename code and UI copy from "component" to "Part" where the glossary now says Part. Wording: the resolution comments on #250, #249, #255, #247, #252, #248 and #251.', 0),
    T('prefactor', 'One kind, one place: move every Effect kind to its own definition',
      'As decided in #255: each kind in its own Go file with its field description, decode and checks, rules text and play capabilities; a lookup table of kinds in place of the shared check constraint; timestamped migrations that may run out of order; a generic recursive Part in the API with definitions served to the builder; the builder form generated from the definition; Scaling replaced by many rules or tables per Effect over scalable fields, with level formulas (#251). The first pull request proves the play direction on three real kinds in a real Session test before the rest move.', 0, ['glossary']),
    T('loader', 'Design files: load them into the database and check them in CI',
      'The reader from #241 for real: strict YAML under `api/db/designs/srd-2024/`, every check it showed failing, and loading into the Effect tables as #242 decides. Includes the per-kind `_no-effect.yaml` lists.', 0, ['prefactor', 'map:242']),
    T('harness', 'Proof harness: fact check, review stamps, pattern registry and coverage ratchet',
      'The five layers decided in #245: the reader (from the loader), the mechanical fact check with `set_aside` and facts declared by each kind, review stamps checked by CI, a pattern registry with one file per pattern and a scenario test each, and the coverage ratchet against the base branch that turns strict per finished kind.', 0, ['loader']),
    T('importer', 'Importer: read Recharge, uses per day, legendary cost and per-slot casting options; apply correction files',
      'As decided in #253: read `usage_limits`, `legendary_action_cost` and `casting_options`; apply one correction file per entry from `api/db/seeds/corrections/` when the snapshot is built, including whole added entries; a snapshot refresh flow that reports changes and flags corrections upstream now matches; a coverage page of upstream reports for the maintainer to file.', 0),
    T('activation', 'Activation: chosen-target Targeting, cost, initial save or attack, duration',
      'As decided in #250 and #247: one Activation path for every owner; legal targets only (count, range, side, creature type, the caster\'s own Senses, willing for the same side), illegal targets blocked with the reason and a logged DM override; cost (slot, action economy including the SRD 5.2 Magic and Utilize actions, Material Components); the initial save or attack with the DC forms `spell`, `ability` and `fixed`; duration and concentration; Scaling applied before targeting; Add-ons offered in the step; applying by hand becomes a DM tool that never counts as full.', 0, ['prefactor']),
    T('feature_link', 'Feature link: derived passive Effects, Feature Resources, Choice pools, granted spells',
      'As decided in #247.', 0, ['activation']),
    T('item_link', 'Item link: derived item Effects, Resources on Item Instances, variants, enforced hidden curses',
      'As decided in #248, including the migration of today\'s Charges.', 0, ['activation']),
    T('monster_link', 'Monster link: derived traits, Combatant Resources, Recharge, the legendary prompt and the lair switch',
      'As decided in #252.', 0, ['activation']),
    T('note', 'Note Part and amounts read from the bearer',
      'The Note Part with its reason from the approved untracked domains (#249, list kept under `docs/rules/`), and values read from the bearer (ability modifier, Proficiency Bonus, class level, a Scale) on the kinds that take a number today.', 0, ['prefactor']),
    T('audit', 'Audit the thirty existing kinds against play, and author the seeded Effects again as Design files',
      'As decided in #243: check each existing kind against play, fold its fixes into the matching family, and replace the twenty seeded Effects with Design files that pass the harness.', 1, ['harness', 'activation']),
]
KIND_TEXT = {
    'one_off': ('Item one-off mechanics', 'The rules only one item has, listed in the items census (`docs/research/census-items.md`, one-off mechanics).'),
    'environment': ('Environment facts: sunlight, underwater, webs, ice, other planes', 'Facts about the surroundings that monster traits read (the monsters census, other gaps: `environment`).'),
}
for g, w in sorted(WAVE.items(), key=lambda kv: (kv[1], kv[0])):
    if w == 0 or g in ('monster_link',):
        continue
    title, extra = KIND_TEXT.get(g, (f'Part kind `{g}`', ''))
    blocked = ['harness', 'activation']
    if g in ('cast_spell', 'spell_rule', 'proficiency', 'resource_rule'):
        blocked.append('feature_link')
    if g in ('sequence',):
        blocked.append('monster_link')
    if g in ('surface+',):
        extra = 'Includes Lightly Obscured, which #249 ruled a gap to build.'
    body = (f'Wave {w}. Merged gap `{g}` from `docs/research/engine-gaps.md` on the `research/engine-gaps` branch; the census names it absorbs are listed there. '
            f'Done when it delivers all four, as decided in #243: play resolution with typed rows and rules text, the generated builder form and Design file field, a scenario test per pattern it enables, and one to three real SRD entries automated with it. ' + extra).strip()
    engine.append(T(g, title, body, w, blocked))
engine.append(T('suggested', 'Suggested Actions know Recharge, area actions and Multiattack',
    'As decided in #252: use a Recharge or area action when it would hit two or more foes the monster sees; prefer Multiattack over a single attack; otherwise today\'s Simple or Cunning rule; observed information only.', 2, ['sequence', 'monster_link']))
keys = {e['key'] for e in engine}

LINK = {'spells': 'activation', 'features': 'feature_link', 'items': 'item_link', 'monsters': 'monster_link'}
GAP_TICKET = {'activation': 'activation', 'note': 'note', 'bearer_amount': 'note', 'feature_link': 'feature_link',
              'item_link': 'item_link', 'monster_link': 'monster_link'}

def chunks(items, size):
    out, cur, w = [], [], 0
    for it in items:
        if cur and w + it['weight'] > size:
            out.append(cur); cur, w = [], 0
        cur.append(it); w += it['weight']
    if cur:
        out.append(cur)
    return out

batches = []
def batch(census, wave, label, members, extra_block=()):
    blocked = {'loader', 'harness', LINK[census]} | set(extra_block)
    for u in members:
        for g in u['gaps']:
            blocked.add(GAP_TICKET.get(g, g))
    batches.append(dict(kind='content', census=census, wave=wave, label=label, members=[u['key'] for u in members],
                        units=sum(u['weight'] for u in members), patterns=sorted({u['pattern'] for u in members}), blocked=sorted(blocked)))

live = [u for u in units if not u['noeffect']]
for census in ('spells', 'features', 'monsters', 'items'):
    for wave in range(5):
        group = [u for u in live if u['census'] == census and u['wave'] == wave and not u['exception']]
        if census == 'monsters':
            group.sort(key=lambda u: u['key'])
            for i, part in enumerate(chunks(group, SIZE)):
                batch(census, wave, 'stat blocks ' + part[0]['key'] + ' to ' + part[-1]['key'], part, ['traits'])
            continue
        group.sort(key=lambda u: (u['pattern'], u['key']))
        for part in chunks(group, SIZE):
            pats = sorted({u['pattern'] for u in part})
            batch(census, wave, ', '.join(pats[:2]) + (f' and {len(pats) - 2} more' if len(pats) > 2 else ''), part)
    exc = [u for u in live if u['census'] == census and u['exception']]
    for part in chunks(sorted(exc, key=lambda u: u['key']), SIZE):
        batch(census, 'exception', 'approved Manual exceptions', part, ['map:244'])

# shared monster traits: one file each under traits/, batched per wave; stat blocks wait on them
import csv as _csv
mrows = list(_csv.DictReader(open('monsters.csv')))
owners = collections.defaultdict(set); tgaps = collections.defaultdict(set)
OTHER = {'trait_owner': 'monster_link', 'legendary': 'monster_link', 'per_day': 'monster_link', 'per_rest': 'monster_link',
         'recharge': 'monster_link', 'cooldown': 'monster_link', 'reaction': 'monster_link', 'activation_cost': 'activation',
         'targeting': 'activation', 'environment': 'environment', 'attack_count': 'sequence'}
def trait_slug(name): return re.sub(r'[^a-z0-9]+', '-', re.sub(r'\(.*?\)', '', name).lower()).strip('-')
for r in mrows:
    if r['kind'] == 'trait':
        owners[trait_slug(r['name'])].add(r['monster'])
        for t in r['kinds_missing'].split(): tgaps[trait_slug(r['name'])] |= eg.merged([t.split(':')[0]], 'monsters')
        for t in r['other_gaps'].split():
            g = OTHER.get(re.sub(r'\(.*', '', t))
            if g: tgaps[trait_slug(r['name'])].add(g)
shared = sorted(k for k, v in owners.items() if len(v) >= 2)
trait_units = [dict(key=k, gaps=sorted(tgaps[k]), weight=1, pattern='shared trait', census='monsters',
                    wave=max([WAVE[g] for g in tgaps[k]], default=0)) for k in shared]
trait_batches = {}
for wave in range(5):
    part = [t for t in trait_units if t['wave'] == wave]
    for p in chunks(part, SIZE):
        batch('monsters', wave, 'shared traits ' + p[0]['key'] + ' to ' + p[-1]['key'], p)
        batches[-1]['traits'] = True
        trait_batches.setdefault(wave, []).append(len(batches) - 1)
for b in batches:
    if b['census'] == 'monsters' and not b.get('traits'):
        w = b['wave'] if b['wave'] != 'exception' else 4
        b['blocked'] = sorted((set(b['blocked']) - {'traits'}) | {f'batch:{i}' for v in range(w + 1) for i in trait_batches.get(v, [])})
# lists and additions that need no kind
batches.append(dict(kind='content', census='features', wave=0, label='Features that need no Effect', units=0, patterns=[],
                    members=[u['key'] for u in units if u['census'] == 'features' and u['noeffect']], blocked=['loader', 'harness'], listing=True))
batches.append(dict(kind='content', census='items', wave=0, label='items with no rules behaviour', units=0, patterns=[],
                    members=['(the 121 rows marked excluded in docs/research/census-items.csv)'], blocked=['loader', 'harness'], listing=True))
batches.append(dict(kind='content', census='items', wave=0, label='the 15 magic items SRD 5.2.1 adds', units=0, patterns=[],
                    members=['(Cloak of Invisibility, Potion of Vitality, Sending Stones and the rest: docs/research/open5e-upstream.md)'], blocked=['importer'], adds=True))
plan = dict(engine=engine, batches=batches)
json.dump(plan, open('plan.json', 'w'), indent=1)
c = collections.Counter((b['census'], b['wave']) for b in batches)
print('engine tickets', len(engine), collections.Counter(e['wave'] for e in engine))
print('content batches', len(batches)); [print(' ', k, v) for k, v in sorted(c.items(), key=str)]
print('waves 0-1 batches', sum(1 for b in batches if b['wave'] in (0, 1)))
print('sample:', json.dumps(batches[3], indent=0)[:600])
