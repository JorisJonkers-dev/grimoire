package vehicles_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
)

func ship() vehicles.Design {
	return vehicles.Design{
		Name: "The Gull", Kind: vehicles.Water, HullMax: 300, Threshold: 15, MilesPerDay: 48,
		Components: []vehicles.Component{{Name: "Sails", HPMax: 100, Drives: true}, {Name: "Helm", HPMax: 50}},
		Stations:   []vehicles.Station{{Name: "Helm", Crew: 1}, {Name: "Rigging", Crew: 6}},
	}
}

// A vehicle has a name, a kind, a hull, a speed, and components and crew stations each named once.
func TestAVehicleIsChecked(t *testing.T) {
	t.Parallel()
	if err := ship().Check(); err != nil {
		t.Fatalf("a sailing ship: %v", err)
	}
	bare := vehicles.Design{Name: "Cart", Kind: vehicles.Land, HullMax: 1, MilesPerDay: 1}
	if err := bare.Check(); err != nil {
		t.Fatalf("a cart with nothing on it: %v", err)
	}
	most := ship()
	most.Name, most.HullMax, most.Threshold, most.MilesPerDay = strings.Repeat("é", vehicles.MaxName), vehicles.MaxHP, vehicles.MaxThreshold, vehicles.MaxMilesPerDay
	most.Components = []vehicles.Component{{Name: strings.Repeat("é", vehicles.MaxName), HPMax: vehicles.MaxHP, Drives: true}}
	most.Stations = []vehicles.Station{{Name: strings.Repeat("é", vehicles.MaxName), Crew: vehicles.MaxCrew}}
	for len(most.Components) < vehicles.MaxParts {
		most.Components = append(most.Components, vehicles.Component{Name: "Oar " + strings.Repeat("i", len(most.Components)), HPMax: 1})
		most.Stations = append(most.Stations, vehicles.Station{Name: "Bench " + strings.Repeat("i", len(most.Stations)), Crew: 0})
	}
	if err := most.Check(); err != nil {
		t.Fatalf("a vehicle at every limit: %v", err)
	}
	for kind := range map[string]bool{vehicles.Land: true, vehicles.Water: true, vehicles.Air: true} {
		d := ship()
		d.Kind = kind
		if err := d.Check(); err != nil {
			t.Fatalf("a %s vehicle: %v", kind, err)
		}
	}

	for want, change := range map[string]func(*vehicles.Design){
		"Give the vehicle a name of up to 80 characters.":    func(d *vehicles.Design) { d.Name = "" },
		"Give the vehicle a name of up to 80 characters. ":   func(d *vehicles.Design) { d.Name = strings.Repeat("é", vehicles.MaxName+1) },
		"A vehicle goes by land, water or air.":              func(d *vehicles.Design) { d.Kind = "rail" },
		"A hull has 1 to 10000 hit points.":                  func(d *vehicles.Design) { d.HullMax = 0 },
		"A hull has 1 to 10000 hit points. ":                 func(d *vehicles.Design) { d.HullMax = vehicles.MaxHP + 1 },
		"A damage threshold runs from 0 to 100.":             func(d *vehicles.Design) { d.Threshold = -1 },
		"A damage threshold runs from 0 to 100. ":            func(d *vehicles.Design) { d.Threshold = vehicles.MaxThreshold + 1 },
		"A vehicle covers 1 to 1000 miles a day.":            func(d *vehicles.Design) { d.MilesPerDay = 0 },
		"A vehicle covers 1 to 1000 miles a day. ":           func(d *vehicles.Design) { d.MilesPerDay = vehicles.MaxMilesPerDay + 1 },
		"A vehicle has up to 20 components.":                 func(d *vehicles.Design) { d.Components = make([]vehicles.Component, vehicles.MaxParts+1) },
		"Give each component a name of up to 80 characters.": func(d *vehicles.Design) { d.Components[0].Name = "" },
		"Give each component a name of up to 80 characters. ": func(d *vehicles.Design) {
			d.Components[1].Name = strings.Repeat("é", vehicles.MaxName+1)
		},
		"Name each component once.":                             func(d *vehicles.Design) { d.Components[1].Name = "Sails" },
		"A component has 1 to 10000 hit points.":                func(d *vehicles.Design) { d.Components[0].HPMax = 0 },
		"A component has 1 to 10000 hit points. ":               func(d *vehicles.Design) { d.Components[1].HPMax = vehicles.MaxHP + 1 },
		"A vehicle has up to 20 crew stations.":                 func(d *vehicles.Design) { d.Stations = make([]vehicles.Station, vehicles.MaxParts+1) },
		"Give each crew station a name of up to 80 characters.": func(d *vehicles.Design) { d.Stations[1].Name = "" },
		"Give each crew station a name of up to 80 characters. ": func(d *vehicles.Design) {
			d.Stations[0].Name = strings.Repeat("é", vehicles.MaxName+1)
		},
		"Name each crew station once.":         func(d *vehicles.Design) { d.Stations[1].Name = "Helm" },
		"A crew station takes 0 to 200 crew.":  func(d *vehicles.Design) { d.Stations[0].Crew = -1 },
		"A crew station takes 0 to 200 crew. ": func(d *vehicles.Design) { d.Stations[1].Crew = vehicles.MaxCrew + 1 },
	} {
		d := ship()
		change(&d)
		var bad vehicles.DesignError
		if err := d.Check(); !errors.As(err, &bad) || err.Error() != strings.TrimSpace(want) {
			t.Errorf("want %q, got %v", strings.TrimSpace(want), err)
		}
	}
}

// Damage under the threshold does nothing; at it or over, all of it lands, never below 0. Repair
// stops at the most a part can have.
func TestDamageAndRepair(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ hp, threshold, amount, want int }{
		{300, 15, 14, 300}, {300, 15, 15, 285}, {300, 15, 16, 284}, {10, 0, 1, 9}, {10, 0, 0, 10}, {10, 5, 30, 0}, {10, 5, 10, 0}, {0, 5, 10, 0},
	} {
		if got := vehicles.Hit(c.hp, c.threshold, c.amount); got != c.want {
			t.Errorf("%d damage against threshold %d at %d: %d, want %d", c.amount, c.threshold, c.hp, got, c.want)
		}
	}
	for _, c := range []struct{ hp, most, amount, want int }{{10, 50, 5, 15}, {10, 50, 40, 50}, {10, 50, 41, 50}, {0, 50, 1, 1}, {50, 50, 0, 50}} {
		if got := vehicles.Mend(c.hp, c.most, c.amount); got != c.want {
			t.Errorf("mending %d at %d of %d: %d, want %d", c.amount, c.hp, c.most, got, c.want)
		}
	}
}

// A vehicle goes at its speed with a whole hull, working drives and a full crew: a wreck goes nowhere,
// each broken drive takes its share, and a short crew halves what is left.
func TestSpeed(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		why  string
		at   vehicles.State
		want int
	}{
		{"whole", vehicles.State{Hull: 300, Drives: 2, Working: 2}, 48},
		{"nothing to drive it but the hull", vehicles.State{Hull: 1}, 48},
		{"a wreck", vehicles.State{Hull: 0, Drives: 2, Working: 2}, 0},
		{"one sail of two", vehicles.State{Hull: 300, Drives: 2, Working: 1}, 24},
		{"one sail of three", vehicles.State{Hull: 300, Drives: 3, Working: 1}, 16},
		{"two sails of three", vehicles.State{Hull: 300, Drives: 3, Working: 2}, 32},
		{"no sail", vehicles.State{Hull: 300, Drives: 2, Working: 0}, 0},
		{"a short crew", vehicles.State{Hull: 300, Drives: 2, Working: 2, ShortHanded: true}, 24},
		{"a short crew and one sail of two", vehicles.State{Hull: 300, Drives: 2, Working: 1, ShortHanded: true}, 12},
		{"a short crew on a wreck", vehicles.State{Hull: 0, ShortHanded: true}, 0},
	} {
		if got := vehicles.Speed(48, c.at); got != c.want {
			t.Errorf("%s: %d miles a day, want %d", c.why, got, c.want)
		}
	}
	if got := vehicles.Speed(1, vehicles.State{Hull: 5, ShortHanded: true}); got != 0 {
		t.Errorf("a short crew on the slowest vehicle: %d", got)
	}
}

// A ship or an airship travels round the clock; a wagon travels a day's eight hours. A leg takes the
// share of a travel day its miles are of a day's miles, and spans whole travel days.
func TestALegAboard(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]int{vehicles.Land: 8, vehicles.Water: 24, vehicles.Air: 24} {
		if got := vehicles.HoursPerDay(kind); got != want {
			t.Errorf("a %s vehicle travels %d hours a day, want %d", kind, got, want)
		}
	}
	for _, c := range []struct {
		kind          string
		miles, perDay int
		minutes, days int
	}{
		{vehicles.Water, 48, 48, 1440, 1},
		{vehicles.Water, 24, 48, 720, 1},
		{vehicles.Water, 49, 48, 1470, 2},
		{vehicles.Water, 96, 48, 2880, 2},
		{vehicles.Land, 24, 24, 480, 1},
		{vehicles.Land, 12, 24, 240, 1},
		{vehicles.Land, 25, 24, 500, 2},
		{vehicles.Land, 1, 7, 69, 1},
		{vehicles.Air, 0, 48, 0, 0},
		{vehicles.Air, 1, 1000, 2, 1},
	} {
		minutes, days := vehicles.Leg(c.miles, c.perDay, c.kind)
		if minutes != c.minutes || days != c.days {
			t.Errorf("%d miles at %d a day by %s: %d minutes over %d days, want %d over %d", c.miles, c.perDay, c.kind, minutes, days, c.minutes, c.days)
		}
	}
	// On the clock a wagon's leg has a night's stop between its travel days; a ship's has none.
	for _, c := range []struct {
		kind                string
		minutes, days, want int
	}{
		{vehicles.Water, 2880, 2, 2880},
		{vehicles.Air, 1470, 2, 1470},
		{vehicles.Land, 480, 1, 480},
		{vehicles.Land, 500, 2, 500 + 960},
		{vehicles.Land, 1440, 3, 1440 + 2*960},
		{vehicles.Land, 0, 0, 0},
		{vehicles.Water, 0, 0, 0},
	} {
		if got := vehicles.Elapsed(c.minutes, c.days, c.kind); got != c.want {
			t.Errorf("%d minutes over %d days by %s: %d on the clock, want %d", c.minutes, c.days, c.kind, got, c.want)
		}
	}
}
