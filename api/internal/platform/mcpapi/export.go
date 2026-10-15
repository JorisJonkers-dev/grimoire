// Package mcpapi serves Grimoire's MCP tools. Each tool is an API operation the spec marks with
// x-mcp, called through the same router as REST so both share auth, validation and errors.
package mcpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Tool is one MCP tool: an API operation an agent may call, with its input schema.
type Tool struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	PathParams  []string `json:"pathParams"`
	QueryParams []string `json:"queryParams"`
	Body        bool     `json:"body"`
	Entity      string   `json:"entity,omitempty"`
	ID          string   `json:"id,omitempty"`
	// Keeps says how a write that records no Revision of a Campaign's prep is kept: "library" as a
	// Revision of a Library entry, "pending" as something that waits for the DM to confirm.
	Keeps string `json:"keeps,omitempty"`
	// Preset is forced into the body and Defaults fill what the caller leaves out, for tools that are
	// one variant of an operation such as a live command.
	Preset      map[string]any  `json:"preset,omitempty"`
	Defaults    map[string]any  `json:"defaults,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type node = map[string]any

// Export reads an OpenAPI spec and returns the tools its operations declare with x-mcp, by name.
func Export(spec []byte) ([]Tool, error) {
	var doc node
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		return nil, err
	}
	paths, _ := doc["paths"].(node)
	var tools []Tool
	for path, item := range paths {
		for method, op := range item.(node) {
			o, ok := op.(node)
			if !ok || o["x-mcp"] == nil {
				continue
			}
			for _, x := range variants(o["x-mcp"].(node)) {
				t, err := tool(doc, path, strings.ToUpper(method), o, x)
				if err != nil {
					return nil, fmt.Errorf("%s %s: %w", method, path, err)
				}
				tools = append(tools, t)
			}
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

// variants lists the tools an operation declares: one, or one per entry of a tools list sharing
// the operation's defaults.
func variants(x node) []node {
	list, ok := x["tools"].([]any)
	if !ok {
		return []node{x}
	}
	out := make([]node, 0, len(list))
	for _, raw := range list {
		v, _ := raw.(node)
		defaults := node{}
		for _, d := range []any{x["defaults"], v["defaults"]} {
			m, _ := d.(node)
			for k, val := range m {
				defaults[k] = val
			}
		}
		v["defaults"] = defaults
		out = append(out, v)
	}
	return out
}

func tool(doc node, path, method string, o, x node) (Tool, error) {
	t := Tool{Method: method, Path: path, PathParams: []string{}, QueryParams: []string{}}
	t.Name, _ = x["tool"].(string)
	t.Entity, _ = x["entity"].(string)
	t.ID, _ = x["id"].(string)
	if t.Name == "" {
		return Tool{}, errors.New("x-mcp needs a tool name")
	}
	t.Description = strings.TrimSpace(fmt.Sprint(o["summary"]) + ". " + fmt.Sprint(o["description"]))
	props, required := node{}, []string{}
	r := &resolver{doc: doc, defs: node{}, recurs: map[string]bool{}}
	params, _ := o["parameters"].([]any)
	for _, raw := range params {
		p, err := r.resolve(raw, nil, 0)
		if err != nil {
			return Tool{}, err
		}
		required = t.param(p.(node), props, required)
	}
	if rb, ok := o["requestBody"].(node); ok {
		content, _ := rb["content"].(node)
		media, _ := content["application/json"].(node)
		body, err := r.resolve(media["schema"], nil, 0)
		if err != nil {
			return Tool{}, err
		}
		if props["body"], err = t.variant(x, body.(node)); err != nil {
			return Tool{}, err
		}
		t.Body = true
		if r, _ := rb["required"].(bool); r {
			required = append(required, "body")
		}
	}
	if method != http.MethodGet && t.Entity == "" && t.Preset == nil && t.Name != "undo_change" && x["keeps"] == nil {
		return Tool{}, errors.New("a write tool needs an entity")
	}
	if err := t.kept(x, method); err != nil {
		return Tool{}, err
	}
	input := node{"type": "object", "additionalProperties": false, "properties": props, "required": required}
	if len(r.defs) > 0 {
		input["$defs"] = r.defs
	}
	schema, err := json.Marshal(input)
	t.InputSchema = schema
	return t, err
}

// Ways a write is kept when it records no Revision of a Campaign's prep.
const (
	KeepsLibrary = "library"
	KeepsPending = "pending"
)

// kept records how a tool's write is kept, and says so where the agent reads it.
func (t *Tool) kept(x node, method string) error {
	if x["keeps"] == nil {
		return nil
	}
	t.Keeps, _ = x["keeps"].(string)
	notes := map[string]string{
		KeepsLibrary: "Every call saves a new Revision of the Library entry, named in the answer; restore_library_revision brings an earlier one back.",
		KeepsPending: "Nothing changes until the DM confirms it in the app.",
	}
	if notes[t.Keeps] == "" || method == http.MethodGet {
		return fmt.Errorf("tool %s: keeps %q is not a way a write is kept", t.Name, t.Keeps)
	}
	t.Description += " " + notes[t.Keeps]
	return nil
}

// variant narrows a body schema to the fields one tool of an operation takes, and records the kind
// it forces and the values it fills in.
func (t *Tool) variant(x, body node) (node, error) {
	kind, ok := x["kind"].(string)
	if !ok {
		return body, nil
	}
	t.Preset, t.Defaults = map[string]any{"kind": kind}, x["defaults"].(node)
	if d, ok := x["description"].(string); ok {
		t.Description = d
	}
	all, _ := body["properties"].(node)
	props := node{}
	fields, _ := x["fields"].([]any)
	for _, f := range fields {
		name := fmt.Sprint(f)
		if all[name] == nil {
			return nil, fmt.Errorf("tool %s: no field %s", t.Name, name)
		}
		props[name] = all[name]
	}
	required, _ := x["required"].([]any)
	for _, r := range required {
		if props[fmt.Sprint(r)] == nil {
			return nil, fmt.Errorf("tool %s: required %v is not one of its fields", t.Name, r)
		}
	}
	out := node{}
	for k, v := range body {
		out[k] = v
	}
	out["properties"], out["required"] = props, append([]any{}, required...)
	delete(out, "description")
	return out, nil
}

// param adds a path or query parameter to the input schema; header parameters stay out.
func (t *Tool) param(p, props node, required []string) []string {
	name, in := p["name"].(string), p["in"]
	if in != "path" && in != "query" {
		return required
	}
	s, _ := p["schema"].(node)
	prop := node{}
	for k, v := range s {
		prop[k] = v
	}
	if d, ok := p["description"].(string); ok {
		prop["description"] = d
	}
	props[name] = prop
	if in == "path" {
		t.PathParams = append(t.PathParams, name)
	} else {
		t.QueryParams = append(t.QueryParams, name)
	}
	if r, _ := p["required"].(bool); r || in == "path" {
		required = append(required, name)
	}
	return required
}

// resolver inlines every $ref below a node, so the tool schema stands alone. A schema that recurs beneath
// a property or an item, as a Part holds Parts, goes once into $defs and is referred to where it recurs.
type resolver struct {
	doc    node
	defs   node
	recurs map[string]bool
}

// resolve inlines v; open holds the schemas being inlined around it, true once a property or an item lies
// between that schema and here.
func (r *resolver) resolve(v any, open map[string]bool, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("schema nests too deep")
	}
	switch x := v.(type) {
	case node:
		return r.resolveNode(x, open, depth)
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			c, err := r.resolve(child, open, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	}
	return v, nil
}

func (r *resolver) resolveNode(x node, open map[string]bool, depth int) (any, error) {
	if ref, ok := x["$ref"].(string); ok {
		return r.ref(ref, open, depth)
	}
	out := node{}
	for k, child := range x {
		if k == "discriminator" || k == "example" {
			continue
		}
		inner := open
		if k == "properties" || k == "items" || k == "additionalProperties" {
			inner = guarded(open)
		}
		c, err := r.resolve(child, inner, depth+1)
		if err != nil {
			return nil, err
		}
		out[k] = c
	}
	return out, nil
}

// ref inlines the schema a $ref names, or refers to it in $defs where it recurs.
func (r *resolver) ref(ref string, open map[string]bool, depth int) (any, error) {
	name := ref[strings.LastIndex(ref, "/")+1:]
	if beneath, ok := open[ref]; ok {
		if !beneath {
			return nil, fmt.Errorf("schema %s is nothing but itself", name)
		}
		r.recurs[ref] = true
		return node{"$ref": "#/$defs/" + name}, nil
	}
	target, err := lookup(r.doc, ref)
	if err != nil {
		return nil, err
	}
	inner := maps.Clone(open)
	if inner == nil {
		inner = map[string]bool{}
	}
	inner[ref] = false
	out, err := r.resolve(target, inner, depth+1)
	if err == nil && r.recurs[ref] {
		r.defs[name] = out
	}
	return out, err
}

// guarded marks every schema being inlined as having a property or an item beneath it.
func guarded(open map[string]bool) map[string]bool {
	out := make(map[string]bool, len(open))
	for k := range open {
		out[k] = true
	}
	return out
}

func lookup(doc node, ref string) (any, error) {
	var cur any = doc
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		m, ok := cur.(node)
		if !ok || m[part] == nil {
			return nil, fmt.Errorf("unresolved $ref %s", ref)
		}
		cur = m[part]
	}
	return cur, nil
}
