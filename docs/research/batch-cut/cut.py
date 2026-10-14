import csv, collections, math, re, sys, json
sys.path.insert(0, '.')
import engine_gaps as eg
SRC = '../census-in'
NOTE_SPELLS = {'druidcraft','elementalism','illusory-script','mending','prestidigitation','purify-food-and-drink','thaumaturgy'}
LINK = {'trait_owner':'monster_link','legendary':'monster_link','per_day':'monster_link','per_rest':'monster_link','recharge':'monster_link',
        'cooldown':'monster_link','attack_count':'sequence','activation_cost':'activation','targeting':'activation','reaction':'monster_link',
        'environment':'environment','seed_data':None}
WAVE = dict(eg.WAVE_OF); WAVE.update({'monster_link':0,'environment':3})
units = []
def add(census, key, gaps, pattern, weight, exc, noeffect=False):
    units.append(dict(census=census, key=key, gaps=sorted(gaps), pattern=pattern or 'unpatterned', weight=weight, exception=exc, noeffect=noeffect,
                      wave=max([WAVE[g] for g in gaps], default=0)))
# spells
for r in eg.table_rows(f'{SRC}/census-spells.md', 'Missing today'):
    if 'Spell' not in r or 'Manual exception' not in r: continue
    slug = r['Spell'].strip('`'); exc = r['Manual exception'].lower().startswith('yes') and slug not in NOTE_SPELLS
    gaps = eg.merged(eg.tokens(r['Missing today'], 'spells'), 'spells')
    if slug in NOTE_SPELLS: gaps = {'note'}
    add('spells', slug, gaps, r['Pattern'], 1, exc)
# features
for r in eg.table_rows(f'{SRC}/census-features.md', 'Missing today'):
    if 'Entry' not in r: continue
    gaps = eg.merged(eg.tokens(r['Missing today'], 'features'), 'features')
    noeff = r['Kinds needed'].strip() in ('-', '–', '')
    add('features', r['Owner'] + ' / ' + r['Entry'].replace('`',''), gaps, r['Pattern'], 1, r['Exception'].lower().startswith('yes'), noeff)
# monsters: per stat block
blocks = collections.defaultdict(lambda: dict(gaps=set(), lines=0, patterns=collections.Counter(), exc=False))
for r in csv.DictReader(open('monsters.csv')):
    b = blocks[r['monster']]; b['lines'] += 1; b['patterns'][r['pattern']] += 1
    b['exc'] |= r['manual_exception'] == 'yes'
    for t in r['kinds_missing'].split():
        b['gaps'] |= eg.merged([t.split(':')[0]], 'monsters')
    for t in r['other_gaps'].split():
        g = LINK.get(re.sub(r'\(.*','',t))
        if g: b['gaps'].add(g)
for slug, b in blocks.items():
    add('monsters', slug, b['gaps'], b['patterns'].most_common(1)[0][0], math.ceil(b['lines']/3), b['exc'])
# items
item_rows = list(csv.DictReader(open('items.csv')))
excluded = sum(1 for r in item_rows if r['template'] == 'excluded' or r['pattern'] == 'excluded')
for r in eg.table_rows(f'{SRC}/census-items.md', 'Missing today'):
    if 'Rows' not in r: continue
    exc = next(v for k, v in r.items() if 'xception' in k)
    name = re.sub(r'\*\*|<br>.*', '', list(r.values())[0])
    add('items', name, eg.merged(eg.tokens(r['Missing today'], 'items'), 'items'), r.get('Pattern',''), 1, exc.lower().startswith('yes'))
json.dump(units, open('units.json','w'), indent=0)
c = collections.Counter((u['census'], 'noeffect' if u['noeffect'] else 'exception' if u['exception'] else f"wave {u['wave']}") for u in units)
for k in sorted(c): print(k, c[k])
print('monster stat blocks', len(blocks), 'lines', sum(b['lines'] for b in blocks.values()), 'excluded items', excluded)
for size in (10, 20, 30):
    n = 0
    groups = collections.defaultdict(int)
    for u in units:
        if u['noeffect']: continue
        groups[(u['census'], 'exc' if u['exception'] else u['wave'], u['pattern'])] += u['weight']
    # pack small pattern groups of same census+wave together
    bins = collections.defaultdict(list)
    for (census, wave, pat), w in groups.items(): bins[(census, wave)].append(w)
    for k, ws in bins.items():
        n += math.ceil(sum(ws) / size)
    print(f'batch size {size}: about {n} content batches (+ no-effect lists, shared traits)')
