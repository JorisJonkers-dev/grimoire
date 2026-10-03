package rules

// TakeDamage applies damage to hit points and temporary hit points: the temporary ones soak it first,
// and hit points stop at 0.
func TakeDamage(hp, temp, amount int) (int, int) {
	amount = max(amount, 0)
	soaked := min(temp, amount)
	return max(hp-(amount-soaked), 0), temp - soaked
}

// Heal restores hit points up to the maximum.
func Heal(hp, maxHP, amount int) int {
	return min(hp+max(amount, 0), maxHP)
}

// GainTempHP grants temporary hit points; they do not stack, so the higher amount stays.
func GainTempHP(temp, amount int) int {
	return max(temp, amount)
}

// Proficiencies are the armour and weapons a class trains a character in (SRD 5.2).
type Proficiencies struct {
	Armor   []string
	Weapons []string
}
