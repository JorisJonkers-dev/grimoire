"""Publish issues.json: milestones, issues (engine all, content waves 0-1), then blocked-by edges. Resumable."""
import json, os, subprocess, tempfile
R = 'JorisJonkers-dev/grimoire'
issues = json.load(open('issues.json'))
state = json.load(open('published.json')) if os.path.exists('published.json') else {}
def gh(*a):
    return subprocess.run(['gh', *a], check=True, capture_output=True, text=True).stdout.strip()
have = set(gh('api', f'repos/{R}/milestones?state=all&per_page=100', '--jq', '.[].title').splitlines())
for t in sorted({x['milestone'] for x in issues}):
    if t not in have:
        gh('api', f'repos/{R}/milestones', '-f', f'title={t}', '-f', 'description=The automation backlog cut by the map #236.')
todo = [x for x in issues if not x['key'].startswith('batch:') or x['wave'] in (0, 1)]
for x in todo:
    if x['key'] in state: continue
    with tempfile.NamedTemporaryFile('w', suffix='.md', delete=False) as f:
        f.write(x['body'])
    args = ['issue', 'create', '--repo', R, '--title', x['title'], '--body-file', f.name, '--milestone', x['milestone']]
    for l in x['labels']: args += ['--label', l]
    url = gh(*args).splitlines()[-1]
    state[x['key']] = int(url.rsplit('/', 1)[1])
    json.dump(state, open('published.json', 'w'), indent=1)
ids = {}
def iid(n):
    if n not in ids: ids[n] = gh('api', f'repos/{R}/issues/{n}', '--jq', '.id')
    return ids[n]
edges = json.load(open('edges.json')) if os.path.exists('edges.json') else []
done = {tuple(e) for e in edges}
for x in todo:
    me = state[x['key']]
    for b in x['blocked']:
        other = int(b.split(':')[1]) if b.startswith('map:') else state.get(b)
        if other is None:
            print('unpublished blocker', x['key'], b); continue
        if (me, other) in done: continue
        gh('api', '-X', 'POST', f'repos/{R}/issues/{me}/dependencies/blocked_by', '-F', f'issue_id={iid(other)}', '--jq', '.number')
        edges.append([me, other]); done.add((me, other))
        json.dump(edges, open('edges.json', 'w'))
print('issues', len(state), 'edges', len(edges), 'range', min(state.values()), '-', max(state.values()))
