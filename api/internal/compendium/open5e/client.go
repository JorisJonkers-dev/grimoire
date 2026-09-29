// Package open5e builds a compendium snapshot from the Open5e v2 API. It runs offline of the
// request path: `grimoire snapshot` writes the file that the binary embeds and imports.
package open5e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

// SRD documents Grimoire imports, with the precedence of the 2024-leads blend.
func srdDocuments() []snapshot.Document {
	return []snapshot.Document{
		{
			Key: "srd-2024", Title: "System Reference Document 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0",
			URL: "https://www.dndbeyond.com/srd",
			Attribution: "This work includes material from the System Reference Document 5.2 (\"SRD 5.2\") by Wizards of the Coast LLC, " +
				"available at https://www.dndbeyond.com/srd. The SRD 5.2 is licensed under the Creative Commons Attribution 4.0 " +
				"International License, available at https://creativecommons.org/licenses/by/4.0/legalcode.",
		},
		{
			Key: "srd-2014", Title: "System Reference Document 5.1", RulesetYear: 2014, Precedence: 10, License: "CC-BY-4.0",
			URL: "https://dnd.wizards.com/resources/systems-reference-document",
			Attribution: "This work includes material taken from the System Reference Document 5.1 (\"SRD 5.1\") by Wizards of the Coast " +
				"LLC and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the " +
				"Creative Commons Attribution 4.0 International License available at https://creativecommons.org/licenses/by/4.0/legalcode.",
		},
	}
}

// Client talks to an Open5e v2 compatible API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Backoff time.Duration
}

type page[T any] struct {
	Next    *string `json:"next"`
	Results []T     `json:"results"`
}

type ref struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type apiSpell struct {
	Key               string   `json:"key"`
	Document          ref      `json:"document"`
	Name              string   `json:"name"`
	Desc              string   `json:"desc"`
	Level             int      `json:"level"`
	HigherLevel       string   `json:"higher_level"`
	School            ref      `json:"school"`
	Classes           []ref    `json:"classes"`
	RangeText         string   `json:"range_text"`
	Range             *float64 `json:"range"`
	RangeUnit         string   `json:"range_unit"`
	Ritual            bool     `json:"ritual"`
	CastingTime       string   `json:"casting_time"`
	Verbal            bool     `json:"verbal"`
	Somatic           bool     `json:"somatic"`
	Material          bool     `json:"material"`
	MaterialSpecified string   `json:"material_specified"`
	SavingThrow       string   `json:"saving_throw_ability"`
	AttackRoll        bool     `json:"attack_roll"`
	DamageRoll        string   `json:"damage_roll"`
	DamageTypes       []string `json:"damage_types"`
	Duration          string   `json:"duration"`
	Concentration     bool     `json:"concentration"`
	CastingOptions    []struct {
		Type       string  `json:"type"`
		DamageRoll *string `json:"damage_roll"`
	} `json:"casting_options"`
}

type apiCondition struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Descriptions []struct {
		Desc     string `json:"desc"`
		Document string `json:"document"`
	} `json:"descriptions"`
}

// Fetch downloads spells and conditions for the SRD documents and maps them to a snapshot.
func (c Client) Fetch(ctx context.Context) (snapshot.Snapshot, error) {
	documents := srdDocuments()
	keys := make([]string, 0, len(documents))
	for _, d := range documents {
		keys = append(keys, d.Key)
	}
	q := url.Values{"document__key__in": {strings.Join(keys, ",")}, "limit": {"100"}}
	spells, err := fetchAll[apiSpell](ctx, c, "/v2/spells/?"+q.Encode())
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	conditions, err := fetchAll[apiCondition](ctx, c, "/v2/conditions/?limit=100")
	if err != nil {
		return snapshot.Snapshot{}, err
	}
	snap := snapshot.Snapshot{Source: "Open5e v2 (" + c.BaseURL + ")", Documents: documents}
	for _, k := range keys {
		if err := c.fetchEntries(ctx, &snap, k); err != nil {
			return snapshot.Snapshot{}, err
		}
	}
	for _, s := range spells {
		snap.Spells = append(snap.Spells, mapSpell(s))
	}
	wanted := map[string]bool{}
	for _, k := range keys {
		wanted[k] = true
	}
	for _, cond := range conditions {
		for _, d := range cond.Descriptions {
			if wanted[d.Document] {
				snap.Conditions = append(snap.Conditions, snapshot.Condition{
					Document: d.Document, Slug: unprefix(cond.Key), Name: cond.Name, Description: strings.TrimSpace(d.Desc),
				})
			}
		}
	}
	sort.Slice(snap.Spells, func(i, j int) bool {
		a, b := snap.Spells[i], snap.Spells[j]
		return a.Document+"/"+a.Slug < b.Document+"/"+b.Slug
	})
	sort.Slice(snap.Conditions, func(i, j int) bool {
		a, b := snap.Conditions[i], snap.Conditions[j]
		return a.Document+"/"+a.Slug < b.Document+"/"+b.Slug
	})
	return snap, snap.Validate()
}

func fetchAll[T any](ctx context.Context, c Client, path string) ([]T, error) {
	var out []T
	next := c.BaseURL + path
	for next != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, fmt.Errorf("open5e: request: %w", err)
		}
		res, err := c.do(req)
		if err != nil {
			return nil, fmt.Errorf("open5e: get %s: %w", next, err)
		}
		var p page[T]
		decodeErr := json.NewDecoder(res.Body).Decode(&p)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("open5e: get %s: status %d", next, res.StatusCode)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("open5e: decode %s: %w", next, decodeErr)
		}
		out = append(out, p.Results...)
		next = ""
		if p.Next != nil {
			next = *p.Next
		}
	}
	return out, nil
}

// do retries server errors a few times; Open5e sits behind a proxy that fails intermittently.
func (c Client) do(req *http.Request) (*http.Response, error) {
	var res *http.Response
	var err error
	for attempt := range 4 {
		if attempt > 0 {
			time.Sleep(c.Backoff * time.Duration(attempt))
		}
		res, err = c.HTTP.Do(req) //nolint:gosec // G704: developer command; URLs are the configured base or its own next links
		if err != nil || res.StatusCode < http.StatusInternalServerError {
			return res, err
		}
		_ = res.Body.Close()
	}
	return res, err
}

func mapSpell(s apiSpell) snapshot.Spell {
	sp := snapshot.Spell{
		Document: s.Document.Key, Slug: unprefix(s.Key), Name: s.Name, Level: s.Level, School: s.School.Key,
		CastingTime: s.CastingTime, RangeText: s.RangeText, Verbal: s.Verbal, Somatic: s.Somatic, Material: s.Material,
		MaterialText: strings.TrimSpace(s.MaterialSpecified), Ritual: s.Ritual, Concentration: s.Concentration,
		Duration: s.Duration, Description: strings.TrimSpace(s.Desc), HigherLevel: strings.TrimSpace(s.HigherLevel),
		SaveAbility: s.SavingThrow, AttackRoll: s.AttackRoll, DamageRoll: s.DamageRoll,
		Classes: []string{}, DamageTypes: []string{}, Scaling: []snapshot.Scaling{},
	}
	if s.Range != nil && s.RangeUnit == "feet" {
		feet := int(*s.Range)
		sp.RangeFeet = &feet
	}
	for _, c := range s.Classes {
		sp.Classes = append(sp.Classes, unprefix(c.Key))
	}
	sp.DamageTypes = append(sp.DamageTypes, s.DamageTypes...)
	for _, o := range s.CastingOptions {
		kind, level, ok := scalingKind(o.Type)
		if ok && o.DamageRoll != nil && *o.DamageRoll != "" {
			sp.Scaling = append(sp.Scaling, snapshot.Scaling{Kind: kind, Level: level, DamageRoll: *o.DamageRoll})
		}
	}
	return sp
}

func scalingKind(t string) (string, int, bool) {
	for prefix, kind := range map[string]string{"slot_level_": "slot", "player_level_": "character"} {
		if rest, found := strings.CutPrefix(t, prefix); found {
			n, err := strconv.Atoi(rest)
			return kind, n, err == nil
		}
	}
	return "", 0, false
}

func unprefix(key string) string {
	if _, after, found := strings.Cut(key, "_"); found {
		return after
	}
	return key
}
