"""Merge the four census documents into one engine gap list.

Usage: engine-gaps.py <dir holding census-{spells,features,monsters,items}.md> <output dir>
The census files live on the research/census-* branches.
"""
import collections
import csv
import re
import sys

# census name -> merged gap. Names differ per census; the definitions decide.
MERGE = {
    'save_gate': 'save_gate', 'dc_source': 'save_gate',
    'attack_gate': 'attack_gate',
    'damage': 'damage', 'slay': 'damage', 'save_damage+': 'damage',
    'heal': 'heal',
    'apply_condition': 'apply_condition',
    'end_effect': 'end_effect', 'remove_condition': 'end_effect', 'dispel+': 'end_effect',
    'trigger': 'trigger', 'timing': 'trigger',
    'modifier': 'modifier', 'ac_modifier': 'modifier', 'attack_modifier': 'modifier', 'hp_max': 'modifier',
    'stat_modifier': 'modifier', 'ability_score': 'modifier', 'stat_override': 'modifier', 'size_change': 'modifier',
    'weapon_change': 'modifier',
    'resistance': 'resistance', 'damage_reduction': 'resistance',
    'condition_immunity': 'immunity', 'immunity': 'immunity',
    'roll_modifier': 'roll_rule', 'roll_adjust': 'roll_rule', 'outcome_change': 'roll_rule', 'check_edge': 'roll_rule',
    'edge_filter': 'roll_rule', 'edge_scope': 'roll_rule', 'save_edge_filter': 'roll_rule', 'edge*': 'roll_rule',
    'save_edge*': 'roll_rule', 'bonus_die*': 'roll_rule',
    'cast_spell': 'cast_spell', 'grant_spell': 'cast_spell', 'item: spell-options': 'cast_spell',
    'spell_change': 'spell_rule', 'slot_change': 'spell_rule', 'spell_store': 'spell_rule', 'reflect_spell': 'spell_rule',
    'branch_condition': 'branch+', 'branch*': 'branch+',
    'sequence': 'sequence',
    'speed': 'speed', 'speed_modifier': 'speed', 'move_mode': 'speed', 'move_rule': 'speed', 'speed_penalty*': 'speed',
    'move': 'movement', 'movement': 'movement', 'forced_move+': 'movement', 'teleport+': 'movement', 'teleport*': 'movement',
    'ends_when': 'duration_rule', 'duration_rule': 'duration_rule',
    'action_economy': 'action_economy', 'restrict': 'action_economy', 'restriction': 'action_economy',
    'turn_order': 'action_economy',
    'sense': 'sense',
    'check_gate': 'check_gate', 'escape_check': 'check_gate', 'item: utilize-check': 'check_gate',
    'grant_proficiency': 'proficiency', 'proficiency': 'proficiency',
    'grant_inspiration': 'resource_rule', 'rest_rule': 'resource_rule', 'resource_change+': 'resource_rule',
    'create_object': 'create_object', 'alter_map': 'alter_map', 'ward': 'ward', 'create_trap': 'create_trap',
    'create_item': 'create_item',
    'area+': 'area+', 'area*': 'area+', 'aura': 'area+',
    'light+': 'light', 'light': 'light',
    'create_surface+': 'surface+', 'create_surface*': 'surface+',
    'banish': 'banish', 'travel': 'banish', 'contain': 'contain',
    'compel': 'compel', 'control': 'compel',
    'conceal': 'conceal', 'grant_quality': 'conceal',
    'detect': 'detect', 'roll_table': 'roll_table',
    'revive': 'revive', 'stabilize': 'revive',
    'summon+': 'summon+', 'summon*': 'summon+', 'form+': 'form+', 'form*': 'form+',
    'item_wear': 'item_wear',
    # wave 0: not component kinds
    'target_filter': 'activation',
    'message': 'note', 'note': 'note',
    'value_source': 'bearer_amount', 'temp_hp+': 'bearer_amount', 'resource_change*': 'bearer_amount',
    'item: charge-scaling': 'bearer_amount',
    'mode_from_choice': 'feature_link',
    'item: use-limits': 'item_link', 'item: depletion': 'item_link', 'item: variant': 'item_link',
    'item: curse-lock': 'item_link', 'item: focus': 'item_link',
}
WAVES = {
    0: ['activation', 'feature_link', 'item_link', 'note', 'bearer_amount'],
    1: ['save_gate', 'trigger', 'modifier', 'damage', 'apply_condition', 'attack_gate', 'resistance', 'heal',
        'immunity', 'end_effect'],
    2: ['roll_rule', 'cast_spell', 'branch+', 'sequence', 'speed', 'movement', 'duration_rule', 'action_economy',
        'grant_action', 'spell_rule', 'sense', 'check_gate', 'proficiency', 'resource_rule'],
    3: ['create_object', 'compel', 'banish', 'area+', 'ward', 'roll_table', 'form+', 'light', 'summon+', 'contain',
        'alter_map', 'conceal', 'detect', 'revive', 'create_item', 'item_wear', 'create_trap', 'surface+'],
    4: ['one_off'],
}
WAVE_NAME = {0: 'Foundation', 1: 'Core resolution', 2: 'Rolls, turns and spells', 3: 'Map and world', 4: 'One-offs'}
WAVE_OF = {gap: wave for wave, gaps in WAVES.items() for gap in gaps}
CENSUSES = ['spells', 'features', 'monsters', 'items']


def table_rows(path, marker):
    out, header = [], None
    for line in open(path, encoding='utf-8'):
        if not line.startswith('|'):
            continue
        cells = [c.strip() for c in line.strip().strip('|').split('|')]
        if marker in cells:
            header = cells
        elif header and len(cells) == len(header) and not set(cells[0]) <= set('-: '):
            out.append(dict(zip(header, cells)))
    return out


def tokens(text, census):
    if census == 'features':
        return [t for t in text.split() if t not in ('-', '–')]
    names = re.findall(r'`([^`]+)`', text)
    if census == 'monsters':
        return [n.split(':')[0] for n in names]
    if census != 'items':
        return names
    mechanics = re.search(r'item:\s*([^;]+)', text)
    if mechanics:
        names += ['item: ' + m.strip() for m in mechanics.group(1).split(',')]
    rest = re.sub(r'`[^`]+`|item:\s*[^;]+', '', text).replace(',', '').replace(';', '').strip(' –-')
    if rest and rest.lower() not in ('none', 'no', 'nothing'):
        names.append('ONE-OFF')
    return names


def merged(names, census):
    gaps = set()
    for name in names:
        if name == 'ONE-OFF':
            gaps.add('one_off')
        elif name == 'grant_action':  # the two censuses mean different things by it
            gaps.add('action_economy' if census == 'features' else 'grant_action')
        else:
            gaps.add(MERGE[name])  # a KeyError here is a census name this script does not know
    return gaps


def load(src):
    units = []

    def add(census, entry, weight, missing, exception):
        gaps = merged(tokens(missing, census), census)
        units.append({'census': census, 'entry': entry, 'weight': weight, 'gaps': gaps,
                      'wave': max([WAVE_OF[g] for g in gaps], default=0), 'exception': exception.lower().startswith('yes')})

    for r in table_rows(f'{src}/census-spells.md', 'Missing today'):
        if 'Spell' in r and 'Manual exception' in r:
            add('spells', r['Spell'].strip('`'), 1, r['Missing today'], r['Manual exception'])
    for r in table_rows(f'{src}/census-features.md', 'Missing today'):
        if 'Entry' in r:
            add('features', r['Owner'] + ' / ' + r['Entry'].replace('`', ''), 1, r['Missing today'], r['Exception'])
    for r in table_rows(f'{src}/census-monsters.md', 'Kinds missing today'):
        if r.get('Entries', '').isdigit():
            add('monsters', re.sub(r'\*\*', '', r['Group (members)']) + ' [' + r['Kind'] + ']', int(r['Entries']),
                r['Kinds missing today'], r['Manual exception'])
    for r in table_rows(f'{src}/census-items.md', 'Missing today'):
        if 'Rows' in r:
            exception = next(v for k, v in r.items() if 'xception' in k)
            add('items', re.sub(r'\*\*|<br>.*', '', list(r.values())[0]), 1, r['Missing today'], exception)
    return units


def main(src, dst):
    units = load(src)
    live = [u for u in units if not u['exception']]
    total = {c: sum(u['weight'] for u in live if u['census'] == c) for c in CENSUSES}
    need = collections.defaultdict(collections.Counter)
    for u in live:
        for g in u['gaps']:
            need[g][u['census']] += u['weight']
    with open(f'{dst}/engine-gaps.csv', 'w', newline='', encoding='utf-8') as f:
        w = csv.writer(f)
        w.writerow(['census', 'entry', 'entries', 'gaps', 'wave', 'exception'])
        for u in units:
            w.writerow([u['census'], u['entry'], u['weight'], ' '.join(sorted(u['gaps'])), u['wave'],
                        'yes' if u['exception'] else 'no'])
    lines = ['| Wave | Gap | Spells | Features | Monsters | Items | All | Census names merged |', '|---|---|---:|---:|---:|---:|---:|---|']
    for wave, gaps in WAVES.items():
        for g in sorted(gaps, key=lambda g: -sum(need[g].values())):
            names = ', '.join(f'`{n}`' for n, m in MERGE.items() if m == g and n != g)
            n = need[g]
            lines.append(f"| {wave} | `{g}` | {n['spells']} | {n['features']} | {n['monsters']} | {n['items']} | {sum(n.values())} | {names} |")
    lines += ['', '| After wave | Spells | Features | Monsters | Items | All |', '|---|---:|---:|---:|---:|---:|',
              f"| *of* | {total['spells']} | {total['features']} | {total['monsters']} | {total['items']} | {sum(total.values())} |"]
    for wave in WAVES:
        done = {c: sum(u['weight'] for u in live if u['census'] == c and u['wave'] <= wave) for c in CENSUSES}
        lines.append(f"| {wave} {WAVE_NAME[wave]} | {done['spells']} | {done['features']} | {done['monsters']} | {done['items']} | {sum(done.values())} |")
    open(f'{dst}/engine-gaps.tables.md', 'w', encoding='utf-8').write('\n'.join(lines) + '\n')
    print(f"{len(units)} rows, {sum(u['weight'] for u in units)} entries, {sum(u['weight'] for u in units) - sum(total.values())} exceptions")


if __name__ == '__main__':
    main(sys.argv[1], sys.argv[2])
