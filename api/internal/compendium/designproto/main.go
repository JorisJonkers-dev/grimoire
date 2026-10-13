// Command designproto is a PROTOTYPE for grimoire issue 241 (Design file format). Throwaway.
//
// It reads every Design file under db/designs/srd-2024, checks the envelope against the Compendium
// seed, and writes one HTML page that shows each file next to the SRD text it automates.
//
//	cd api && go run ./internal/compendium/designproto
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	root = "db/designs/srd-2024"
	doc  = "srd-2024"
)

type design struct {
	Entry      string         `yaml:"entry" json:"entry"`
	Name       string         `yaml:"name" json:"name"`
	Automation string         `yaml:"automation" json:"automation"`
	Exception  string         `yaml:"exception,omitempty" json:"exception,omitempty"`
	Pattern    string         `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Attunement bool           `yaml:"attunement,omitempty" json:"attunement,omitempty"`
	Params     []string       `yaml:"params,omitempty" json:"params,omitempty"`
	Legendary  map[string]any `yaml:"legendary,omitempty" json:"legendary,omitempty"`
	AppliesTo  []applies      `yaml:"applies_to,omitempty" json:"appliesTo,omitempty"`
	Effects    []effect       `yaml:"effects" json:"effects"`
}

type applies struct {
	Match    string `yaml:"match" json:"match"`
	Category string `yaml:"category,omitempty" json:"category,omitempty"`
	Bonus    []int  `yaml:"bonus,omitempty" json:"bonus,omitempty"`
}

type effect struct {
	Of         string           `yaml:"of,omitempty" json:"of,omitempty"`
	Label      string           `yaml:"label,omitempty" json:"label,omitempty"`
	Use        string           `yaml:"use,omitempty" json:"use,omitempty"`
	With       map[string]any   `yaml:"with,omitempty" json:"with,omitempty"`
	Attack     string           `yaml:"attack,omitempty" json:"attack,omitempty"`
	OnHit      []map[string]any `yaml:"on_hit,omitempty" json:"onHit,omitempty"`
	Activation map[string]any   `yaml:"activation,omitempty" json:"activation,omitempty"`
	Parts      []map[string]any `yaml:"parts,omitempty" json:"parts,omitempty"`
	Scaling    []map[string]any `yaml:"scaling,omitempty" json:"scaling,omitempty"`
}

type seed struct {
	Spells []struct {
		Document, Slug, Name, Description string
		HigherLevel                       string `json:"higherLevel"`
	}
	Classes []struct {
		Document, Slug string
		Features       []struct{ Slug, Name, Description string }
	}
	Species []struct {
		Document, Slug string
		Traits         []struct{ Name, Description string }
	}
	Feats []struct {
		Document, Slug, Name string
		Benefits             []string
	}
	Monsters []struct {
		Document, Slug, Name string
		Actions              []struct{ Name, Description, Type string }
		Traits               []struct{ Name, Description string }
	}
	Items []struct {
		Document, Slug, Name, Category, Description string
		RequiresAttunement                          bool `json:"requiresAttunement"`
	}
}

type srd struct{ Title, Text string }

type page struct {
	Path, YAML, JSON string
	Problems         []string
	Facts            []string
	SRD              []srd
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

var noteReasons = map[string]bool{"narration": true, "languages and speech": true, "breathing and suffocation": true,
	"cosmetic appearance": true, "sustenance, sleep and aging": true, "beyond the session": true,
	"truth and free social constraint": true, "minor object state": true}

func main() {
	var s seed
	f, err := os.Open("db/seeds/compendium.json.gz")
	must(err)
	z, err := gzip.NewReader(f)
	must(err)
	must(json.NewDecoder(z).Decode(&s))

	files := map[string]design{}
	var pages []page
	must(filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return err
		}
		raw, err := os.ReadFile(p)
		must(err)
		var x design
		dec := yaml.NewDecoder(bytes.NewReader(raw))
		dec.KnownFields(true)
		rel := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(p), root+"/"), ".yaml")
		pg := page{Path: rel + ".yaml", YAML: string(raw)}
		if err := dec.Decode(&x); err != nil {
			pg.Problems = append(pg.Problems, "does not parse: "+err.Error())
		} else {
			files[rel] = x
			j, _ := json.MarshalIndent(x, "", "  ")
			pg.JSON = string(j)
		}
		pages = append(pages, pg)
		return nil
	}))

	for i := range pages {
		rel := strings.TrimSuffix(pages[i].Path, ".yaml")
		x, ok := files[rel]
		if !ok {
			continue
		}
		pg := &pages[i]
		bad := func(format string, a ...any) { pg.Problems = append(pg.Problems, fmt.Sprintf(format, a...)) }
		if x.Entry != rel {
			bad("entry %q does not match the file's path %q", x.Entry, rel)
		}
		switch x.Automation {
		case "full":
			if x.Exception != "" {
				bad("a full entry carries no exception")
			}
		case "exception":
			if x.Exception == "" {
				bad("an exception needs its reason")
			}
		default:
			bad("automation is full or exception, not %q", x.Automation)
		}
		kind, rest, _ := strings.Cut(rel, "/")
		name := ""
		switch kind {
		case "spells":
			for _, sp := range s.Spells {
				if sp.Document == doc && sp.Slug == rest {
					name = sp.Name
					pg.SRD = append(pg.SRD, srd{sp.Name, sp.Description}, srd{"Using a higher-level spell slot", sp.HigherLevel})
				}
			}
		case "features":
			class, feature, _ := strings.Cut(rest, "/")
			for _, c := range s.Classes {
				for _, ft := range c.Features {
					if c.Document == doc && c.Slug == class && ft.Slug == feature {
						name = ft.Name
						pg.SRD = append(pg.SRD, srd{ft.Name, ft.Description})
					}
				}
			}
		case "species":
			species, trait, _ := strings.Cut(rest, "/")
			for _, sp := range s.Species {
				for _, t := range sp.Traits {
					if sp.Document == doc && sp.Slug == species && slugify(t.Name) == trait {
						name = t.Name
						pg.SRD = append(pg.SRD, srd{t.Name, t.Description})
					}
				}
			}
		case "feats":
			for _, ft := range s.Feats {
				if ft.Document == doc && ft.Slug == rest {
					name = ft.Name
					pg.SRD = append(pg.SRD, srd{ft.Name, strings.Join(ft.Benefits, "\n\n")})
				}
			}
		case "monsters":
			for _, m := range s.Monsters {
				if m.Document != doc || m.Slug != rest {
					continue
				}
				name = m.Name
				covered := map[string]bool{}
				for _, e := range x.Effects {
					covered[e.Of] = true
				}
				in := map[string]bool{}
				for _, a := range m.Actions {
					in[a.Name] = true
					pg.SRD = append(pg.SRD, srd{a.Name + " (" + strings.ReplaceAll(a.Type, "_", " ") + ")", a.Description})
					if !covered[a.Name] {
						bad("the stat block's %q is not accounted for", a.Name)
					}
				}
				for _, t := range m.Traits {
					in[t.Name] = true
					pg.SRD = append(pg.SRD, srd{t.Name + " (trait)", t.Description})
					if !covered[t.Name] {
						bad("the stat block's %q is not accounted for", t.Name)
					}
				}
				for _, e := range x.Effects {
					if !in[e.Of] {
						bad("%q is not an action or trait of this stat block", e.Of)
					}
				}
			}
		case "traits":
			name = x.Name
			n, text := 0, ""
			for _, m := range s.Monsters {
				for _, t := range m.Traits {
					if m.Document == doc && strings.HasPrefix(slugify(t.Name), rest) {
						n++
						text = t.Description
					}
				}
			}
			pg.Facts = append(pg.Facts, strconv.Itoa(n)+" stat blocks carry this trait")
			pg.SRD = append(pg.SRD, srd{x.Name + " (as one stat block words it)", text})
			if n == 0 {
				bad("no stat block carries this trait")
			}
		case "items":
			name = x.Name
			var hit []string
			for _, a := range x.AppliesTo {
				bonuses := a.Bonus
				if len(bonuses) == 0 {
					bonuses = []int{0}
				}
				for _, b := range bonuses {
					pattern := strings.ReplaceAll(a.Match, "{bonus}", strconv.Itoa(b))
					for _, it := range s.Items {
						if ok, _ := path.Match(pattern, it.Slug); ok && it.Document == doc && (a.Category == "" || a.Category == it.Category) {
							hit = append(hit, it.Slug)
							if it.RequiresAttunement != x.Attunement {
								bad("%s: attunement in the seed is %v", it.Slug, it.RequiresAttunement)
							}
							if len(hit) == 1 {
								pg.SRD = append(pg.SRD, srd{it.Name, it.Description})
							}
						}
					}
				}
			}
			sort.Strings(hit)
			pg.Facts = append(pg.Facts, fmt.Sprintf("applies to %d seed items", len(hit)))
			if len(hit) > 0 {
				pg.Facts = append(pg.Facts, "first: "+hit[0]+", last: "+hit[len(hit)-1])
			} else {
				bad("applies to no seed item")
			}
		default:
			bad("unknown kind of entry %q", kind)
		}
		switch {
		case name == "":
			bad("no such entry in the seed")
		case kind != "traits" && kind != "items" && name != x.Name:
			bad("name %q is %q in the seed", x.Name, name)
		}
		if len(x.Effects) == 0 {
			bad("no effects")
		}
		for n, e := range x.Effects {
			ways := 0
			for _, has := range []bool{e.Use != "", e.Attack != "", len(e.Parts) > 0} {
				if has {
					ways++
				}
			}
			if ways != 1 {
				bad("effect %d states exactly one of use, attack or parts", n+1)
			}
			if e.Use != "" {
				shared, ok := files[e.Use]
				if !ok {
					bad("effect %d uses %q, which is not a file", n+1, e.Use)
				}
				for _, p := range shared.Params {
					if _, given := e.With[p]; !given {
						bad("effect %d uses %q without its %q", n+1, e.Use, p)
					}
				}
			}
			walk(append(e.Parts, e.OnHit...), func(part map[string]any) {
				switch part["kind"] {
				case nil:
					bad("effect %d has a Part with no kind", n+1)
				case "manual":
					if x.Automation == "full" {
						bad("a full entry holds a Manual Effect")
					}
				case "note":
					if reason, _ := part["reason"].(string); !noteReasons[reason] {
						bad("a Note needs a reason from the approved list, not %q", reason)
					}
				}
			})
		}
	}

	sort.Slice(pages, func(i, j int) bool { return pages[i].Path < pages[j].Path })
	problems := 0
	for _, p := range pages {
		status := "ok"
		if len(p.Problems) > 0 {
			status = strings.Join(p.Problems, "; ")
			problems += len(p.Problems)
		}
		fmt.Printf("%-40s %s %s\n", p.Path, strings.Join(p.Facts, ", "), status)
	}
	fmt.Printf("%d files, %d problems\n", len(pages), problems)
	out, err := os.Create("../docs/prototypes/design-files.html")
	must(err)
	must(template.Must(template.New("page").Parse(tmpl)).Execute(out, pages))
	must(out.Close())
}

// walk visits every Part, including the Parts held under another Part's named groups.
func walk(parts []map[string]any, visit func(map[string]any)) {
	for _, p := range parts {
		visit(p)
		for _, v := range p {
			list, _ := v.([]any)
			var under []map[string]any
			for _, item := range list {
				if m, ok := item.(map[string]any); ok && m["kind"] != nil {
					under = append(under, m)
				}
			}
			walk(under, visit)
		}
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

const tmpl = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>PROTOTYPE: Design files</title><style>
:root{--bg:#15130f;--panel:#1e1b15;--line:#3a3326;--ink:#e9e2d0;--dim:#a39a84;--gold:#c9a24a;--bad:#d9705a;--ok:#7fb37a}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--ink);font:15px/1.5 -apple-system,"Segoe UI",sans-serif}
main{max-width:1320px;margin:0 auto;padding:32px 40px 80px}h1{font-size:26px;margin:0 0 6px}h2{font-size:16px;margin:0;font-family:ui-monospace,Menlo,monospace;color:var(--gold)}
h3{font-size:13px;margin:12px 0 2px;color:var(--dim);font-weight:normal;text-transform:uppercase;letter-spacing:.05em}
.tag{color:var(--bad);font-size:12px;letter-spacing:.08em}.dim{color:var(--dim)}p{margin:0 0 8px}
section{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:16px 20px;margin-top:18px}
.cols{display:grid;grid-template-columns:1fr 1fr;gap:24px}pre{margin:0;white-space:pre-wrap;font:13px/1.5 ui-monospace,Menlo,monospace}
.head{display:flex;justify-content:space-between;align-items:baseline;margin-bottom:6px}.ok{color:var(--ok)}.bad{color:var(--bad)}
button{background:transparent;color:var(--ink);border:1px solid var(--line);border-radius:5px;padding:6px 12px;font:inherit;cursor:pointer}button.on{border-color:var(--gold);color:var(--gold)}
nav{position:sticky;top:0;background:var(--bg);padding:10px 0;border-bottom:1px solid var(--line);display:flex;gap:8px;align-items:center;flex-wrap:wrap}
nav a{color:var(--dim);font-size:13px;text-decoration:none;margin-right:6px}nav a:hover{color:var(--gold)}
body.json .yaml,body:not(.json) .json{display:none}
</style></head><body><main>
<div class="tag">PROTOTYPE · THROWAWAY · grimoire issue 241</div>
<h1>Design files, read back against the SRD text</h1>
<p class="dim" style="max-width:920px">The question: what does one Design file look like for a spell, a Feature, a monster and an item, and where does it live? Each file below is shown as written, next to the seed text it automates, with what a reader checked: that the file's path names a real entry, that every action and trait of a stat block is accounted for, that an item file reaches real seed items, that a Note states its reason, and that a full entry holds no Manual Effect. Fields inside a Part are illustrative.</p>
<nav><span class="dim">Show files as</span><button id="y" class="on" onclick="document.body.classList.remove('json');y.classList.add('on');j.classList.remove('on')">YAML</button><button id="j" onclick="document.body.classList.add('json');j.classList.add('on');y.classList.remove('on')">JSON</button>
<span style="flex:1"></span>{{range .}}<a href="#{{.Path}}">{{.Path}}</a>{{end}}</nav>
{{range .}}<section id="{{.Path}}"><div class="head"><h2>{{.Path}}</h2>
{{if .Problems}}<span class="bad">{{len .Problems}} problem(s)</span>{{else}}<span class="ok">checks pass</span>{{end}}</div>
{{range .Facts}}<p class="dim">{{.}}</p>{{end}}{{range .Problems}}<p class="bad">{{.}}</p>{{end}}
<div class="cols"><div><h3>Design file</h3><pre class="yaml">{{.YAML}}</pre><pre class="json">{{.JSON}}</pre></div>
<div><h3>Seed text</h3>{{range .SRD}}{{if .Text}}<p><strong>{{.Title}}.</strong> {{.Text}}</p>{{end}}{{end}}</div></div></section>{{end}}
</main></body></html>`
