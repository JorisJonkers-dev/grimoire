// Command kindproto-dump prints every kind's definition as JSON, and proves a part can be decoded,
// described and folded into play without a switch over kinds. PROTOTYPE for grimoire issue 255.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects/kindproto"
)

func main() {
	var out []kindproto.Description
	bySlug := map[string]kindproto.Kind{}
	for _, k := range kindproto.Kinds() {
		out = append(out, k.Describe())
		bySlug[k.Describe().Slug] = k
	}
	if len(os.Args) > 1 && os.Args[1] == "check" {
		bless, err := bySlug["bonus_die"].Decode(kindproto.Values{"dice": "1d4", "on": []string{"attack rolls", "saving throws"}})
		fmt.Println(bless.Text(nil), err)
		var p kindproto.AttackProfile
		if f, ok := bless.(kindproto.Folder); ok {
			f.FoldAttack(&p)
		}
		fmt.Println("attack profile:", p.BonusDice)
		cure, _ := bySlug["heal"].Decode(kindproto.Values{"dice": "2d8", "bonus": "spellcasting ability modifier"})
		fmt.Println(cure.Text(nil), cure.(kindproto.Lander).Land(kindproto.Target{Name: "Edda"}))
		_, err = bySlug["save_damage"].Decode(kindproto.Values{"ability": "dexterity", "dice": "lots", "damage_type": "fire"})
		fmt.Println("rejected:", err)
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	_ = enc.Encode(out)
}
