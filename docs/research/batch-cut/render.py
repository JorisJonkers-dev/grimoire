"""Render plan.json into issue titles and bodies (no network)."""
import csv, json, re, collections
plan = json.load(open('plan.json'))
templates = {r['template_name']: r['template'] for r in csv.DictReader(open('items.csv'))}
slug = lambda s: re.sub(r'[^a-z0-9]+', '-', s.lower()).strip('-')
def path(census, key):
    if census == 'spells': return f'spells/{key}'
    if census == 'monsters': return f'monsters/{key}'
    if census == 'items': return f'items/{templates.get(key, slug(key))}'
    owner, entry = key.split(' / ', 1)
    word, rest = owner.split(' ', 1)
    if word == 'class': return f'features/{rest}/{entry.split()[0]}'
    if word == 'subclass':
        sub = rest.split(' ')[0]; e = entry.split()[0]
        return f'features/{sub}/{e.split("_", 1)[1] if "_" in e else e}'
    if word == 'species': return f'species/{rest}/{slug(entry)}'
    return f'feats/{entry.split()[0]}'
W = {0: 'Automation wave 0', 1: 'Automation wave 1', 2: 'Automation wave 2', 3: 'Automation wave 3', 4: 'Automation wave 4', 'exception': 'Automation wave 4'}
PRI = {0: 'priority: P1', 1: 'priority: P1', 2: 'priority: P2', 3: 'priority: P2', 4: 'priority: P3', 'exception': 'priority: P3'}
issues = []
for e in plan['engine']:
    t = 'type: docs' if e['key'] == 'glossary' else 'type: refactor' if e['key'] == 'prefactor' else 'type: feature'
    body = f"## What\n\n{e['body']}\n\n## Part of\n\nThe automation backlog cut by the map #236; the decisions it rests on are listed there.\n"
    issues.append(dict(key=e['key'], title=e['title'], body=body, labels=[t, PRI[e['wave']]], milestone=W[e['wave']], blocked=e['blocked'], wave=e['wave']))
counts = collections.Counter()
groups = collections.Counter((b['census'], b['wave'], b['label']) for b in plan['batches'])
for i, b in enumerate(plan['batches']):
    k = (b['census'], b['wave'], b['label']); counts[k] += 1
    n = f" ({counts[k]} of {groups[k]})" if groups[k] > 1 else ''
    wave = 'exceptions' if b['wave'] == 'exception' else f"wave {b['wave']}"
    title = f"Automate {wave} {b['census']}: {b['label']}{n}"
    if b.get('listing'):
        title = f"List the {b['label']}"
        entries = '\n'.join(f'- `{path(b["census"], m) if " / " in m else m}`' for m in b['members'])
        do = f"Write `{b['census']}/_no-effect.yaml`, naming each entry below with its reason (class table column, Choice, sheet value, plain gear), as decided in #241. This file is this batch's alone."
    elif b.get('adds'):
        title = f"Add {b['label']} as correction files"
        entries = '\n'.join(f'- {m}' for m in b['members'])
        do = 'Add each item as a correction file with its SRD 5.2.1 text and page (#253), then census its Parts the way `docs/research/census-items.md` does and report which wave each needs. Their Design files follow in a later cut.'
    else:
        entries = '\n'.join(f'- `{path(b["census"], m) if not b.get("traits") else "traits/" + m}`' for m in b['members'])
        branch = f"automate/{slug(wave)}-{b['census']}-{i}"
        do = (f"1. Work in a worktree on branch `{branch}`.\n"
              f"2. Write one Design file per entry under `api/db/designs/srd-2024/`; skip an entry that already has one (a kind ticket's tracer). Add a correction file where the seed text is known to be wrong (#253).\n"
              f"3. Patterns: {', '.join(b['patterns'])}. Use each pattern's test if it exists; if this batch is a pattern's first, write its test and its registry file.\n"
              f"4. `task check` green: the reader, the fact check, a reason for every `set_aside`.\n"
              f"5. Ask the reviewer agent (Sonnet) for stamps.\n"
              f"6. Open the PR. Its body names three random entries, with the generated rules text beside the SRD text.")
    body = (f"## Entries ({b['units']} units)\n\n{entries}\n\n## Do\n\n{do}\n\n"
            "## Never touch\n\nAnother batch's Design or correction files, the list of kinds, engine code. A missing kind means stop and report on this issue.\n\n"
            "## Done when\n\nCI is green, the stamps are fresh, and the maintainer approves the three samples.\n\n"
            "## Part of\n\nThe automation backlog cut by the map #236.\n")
    issues.append(dict(key=f'batch:{i}', title=title, body=body, labels=['type: feature', PRI[b['wave']]], milestone=W[b['wave']], blocked=b['blocked'], wave=b['wave']))
json.dump(issues, open('issues.json', 'w'), indent=1)
now = [x for x in issues if x['wave'] in (0, 1)]
print(len(issues), 'rendered;', len(now), 'to publish now')
print(max(len(x['title']) for x in issues), 'longest title')
x = next(x for x in now if x['key'].startswith('batch:') and 'spells' in x['title'])
print(x['title'], x['labels'], x['milestone'], x['blocked']); print(x['body'][:1200])
