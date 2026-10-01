// Package mcpapi serves Grimoire's MCP tools. Each tool is an API operation the spec marks with
// x-mcp, called through the same router as REST so both share auth, validation and errors.
package mcpapi

import (
	"encoding/json"
	"errors"
	"fmt"
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
	params, _ := o["parameters"].([]any)
	for _, raw := range params {
		p, err := resolve(doc, raw, 0)
		if err != nil {
			return Tool{}, err
		}
		required = t.param(p.(node), props, required)
	}
	if rb, ok := o["requestBody"].(node); ok {
		content, _ := rb["content"].(node)
		media, _ := content["application/json"].(node)
		body, err := resolve(doc, media["schema"], 0)
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
	if method != http.MethodGet && t.Entity == "" && t.Preset == nil && t.Name != "undo_change" {
		return Tool{}, errors.New("a write tool needs an entity")
	}
	schema, err := json.Marshal(node{"type": "object", "additionalProperties": false, "properties": props, "required": required})
	t.InputSchema = schema
	return t, err
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

// resolve inlines every $ref below a node, so the tool schema stands alone.
func resolve(doc node, v any, depth int) (any, error) {
	if depth > 32 {
		return nil, errors.New("schema nests too deep")
	}
	switch x := v.(type) {
	case node:
		return resolveNode(doc, x, depth)
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			r, err := resolve(doc, child, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	}
	return v, nil
}

func resolveNode(doc, x node, depth int) (any, error) {
	if ref, ok := x["$ref"].(string); ok {
		target, err := lookup(doc, ref)
		if err != nil {
			return nil, err
		}
		return resolve(doc, target, depth+1)
	}
	out := node{}
	for k, child := range x {
		if k == "discriminator" || k == "example" {
			continue
		}
		r, err := resolve(doc, child, depth+1)
		if err != nil {
			return nil, err
		}
		out[k] = r
	}
	return out, nil
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
