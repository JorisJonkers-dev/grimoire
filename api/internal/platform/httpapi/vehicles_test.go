package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vehicles"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

type brokenVehicles struct{ err error }

func (b brokenVehicles) List(context.Context, caller.Caller, uuid.UUID) (playapp.VehiclesView, error) {
	return playapp.VehiclesView{}, b.err
}

func (b brokenVehicles) Add(context.Context, caller.Caller, uuid.UUID, vehicles.Design) (playdomain.Vehicle, error) {
	return playdomain.Vehicle{}, b.err
}

func (b brokenVehicles) Remove(context.Context, caller.Caller, uuid.UUID, uuid.UUID) error {
	return b.err
}

func (b brokenVehicles) Strike(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.VehicleBlow) (playdomain.Vehicle, error) {
	return playdomain.Vehicle{}, b.err
}

func (b brokenVehicles) Post(context.Context, caller.Caller, uuid.UUID, uuid.UUID, uuid.UUID, int) (playdomain.Vehicle, error) {
	return playdomain.Vehicle{}, b.err
}

// A fault in the vehicle service is a fault, and nobody signed out gets anywhere.
func TestAFailingVehicleServiceAnswers503WithoutItsCause(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/vehicles"
	one := base + "/0190c7a8-0000-7000-8000-000000000002"
	h := campaignServer(t, brokenCampaigns{}, httpapi.VehicleService(brokenVehicles{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, `{"name":"A","kind":"land","hullMax":1,"milesPerDay":1}`},
		{http.MethodDelete, one, ""},
		{http.MethodPost, one + "/damage", `{"amount":1}`},
		{http.MethodPut, one + "/stations/0190c7a8-0000-7000-8000-000000000003", `{"posted":1}`},
	} {
		t.Run(o.method+" "+o.path, func(t *testing.T) {
			if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
				t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestVehicleOperationsWithoutAnIdentityAnswer401(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListVehicles(ctx, oas.ListVehiclesParams{}))
	add(hh.CreateVehicle(ctx, &oas.VehicleInput{}, oas.CreateVehicleParams{}))
	add(hh.DeleteVehicle(ctx, oas.DeleteVehicleParams{}))
	add(hh.DamageVehicle(ctx, &oas.VehicleBlow{}, oas.DamageVehicleParams{}))
	add(hh.PostVehicleCrew(ctx, &oas.VehicleCrew{}, oas.PostVehicleCrewParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
