package live_test

import (
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

var update = flag.Bool("update", false, "rewrite the shared live message fixture")

const fixture = "../../../../fixtures/live-messages.json"

type contract struct {
	Commands []live.Command `json:"commands"`
	Updates  []live.Update  `json:"updates"`
}

// samples covers every command and update kind; the web client parses each with its generated schemas.
func samples() contract {
	token := live.TokenView{ID: "0190c7a8-0000-7000-8000-00000000000a", Label: "Goblin", Kind: "enemy", Q: 2, R: -1, Hidden: false}
	return contract{
		Commands: []live.Command{
			{Nonce: "n1", Kind: live.CmdResync},
			{Nonce: "n2", Kind: live.CmdPlace, Label: "Goblin", TokenKind: "enemy", Q: 2, R: -1, Hidden: true},
			{Nonce: "n3", Kind: live.CmdMove, TokenID: token.ID, Q: 3, R: -1},
			{Nonce: "n4", Kind: live.CmdSetHidden, TokenID: token.ID, Hidden: false},
			{Nonce: "n5", Kind: live.CmdRemove, TokenID: token.ID},
		},
		Updates: []live.Update{
			{
				Kind: live.UpdSnapshot, Seq: 4, Tokens: []live.TokenView{token},
				Session: &live.SessionView{ID: "0190c7a8-0000-7000-8000-00000000000b", Number: 3, GridRadius: 10, Audience: live.AudienceParty},
			},
			{Kind: live.UpdToken, Seq: 5, Nonce: "n3", Token: &token},
			{Kind: live.UpdTokenRemoved, Seq: 6, TokenID: token.ID},
			{Kind: live.UpdTick, Seq: 7},
			{Kind: live.UpdRejected, Seq: 7, Nonce: "n9", Reason: "Only the DM can change tokens."},
			{Kind: live.UpdEnded, Seq: 7},
		},
	}
}

// TestContractFixture pins the Go wire format to the fixture the web client validates.
func TestContractFixture(t *testing.T) {
	got := samples()
	if *update {
		raw, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixture, append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var want contract
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("live messages changed; rerun with -update and check the web client still parses them")
	}
}
