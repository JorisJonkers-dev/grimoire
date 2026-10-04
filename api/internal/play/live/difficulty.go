package live

import (
	"context"
	"fmt"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/difficulty"
)

// preset reads the Campaign's difficulty preset now. The DM changes it outside the Session, so it is
// read as it is needed; a preset that cannot be read plays by the rules as written.
func (r *runtime) preset() difficulty.Preset {
	slug, err := r.store.Difficulty(context.Background(), r.campaign)
	if err != nil {
		r.log.Error("live: difficulty", "error", err)
		return difficulty.Of(difficulty.Standard)
	}
	return difficulty.Of(slug)
}

// toughened gives an enemy the hit points the Campaign's difficulty preset has it come onto the map
// with. Nobody else's change, and a creature keeps what it came with.
func (r *runtime) toughened(kind string, stats *domain.Stats) {
	if stats == nil || kind != domain.TokenEnemy {
		return
	}
	p := r.preset()
	stats.HP, stats.HPMax = p.HitPoints(stats.HP), p.HitPoints(stats.HPMax)
}

// attackModifier is what the Campaign's difficulty preset adds to an attack roll of an enemy, named
// for its Roll Card; nothing for anybody else's.
func (r *runtime) attackModifier(attacker domain.Token) domain.Modifier {
	if attacker.Kind != domain.TokenEnemy {
		return domain.Modifier{Label: "", Value: 0}
	}
	p := r.preset()
	return domain.Modifier{Label: p.Name + " difficulty", Value: p.ToHit}
}

// difficulty adds what the preset gives an enemy's attack to the attack being aimed.
func (r *runtime) difficulty(p *aim) {
	if m := r.attackModifier(p.attacker); m.Value != 0 {
		p.preset, p.reasons = m, append(p.reasons, fmt.Sprintf("%s: %+d to hit", m.Label, m.Value))
	}
}
