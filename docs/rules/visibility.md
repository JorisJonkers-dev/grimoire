# Visibility Qualities and Senses

The rules engine decides what reaches Party Vision by checking each creature's and object's Visibility Qualities against the Senses of the party, within range. This table is the reference the engine implements; it is not shown in the Homebrew editors. The glossary's Obscured quality has the two forms below: darkness and heavy obscurement.

| Quality | Sight | Darkvision | Blindsight | Tremorsense | Truesight |
|---|---|---|---|---|---|
| **Hidden** (unseen, unheard) | check (Perception vs Stealth) | check (Perception vs Stealth) | within range (if within range) | within range (if on the ground) | check (Perception vs Stealth) |
| **Invisible** (magic or nature) | no | no | within range | within range (if on the ground) | sees it |
| **Disguised** (false identity) | check (Insight or Investigation) | check (Insight or Investigation) | no | no | sees it (true form) |
| **Illusory** (not really there) | check (Investigation) | check (Investigation) | sees it | sees it | sees it |
| **Ethereal** (on the Border Ethereal) | no | no | no | no | sees it |
| **Obscured: darkness** (no light) | no | sees it (as dim light) | within range | within range (if on the ground) | sees it |
| **Obscured: heavy** (fog, foliage) | no | no | within range | within range (if on the ground) | no |
| **Secret** (objects: doors, caches) | check (Perception or Investigation) | check (Perception or Investigation) | no | no | no |

- **check** means the quality holds until a check beats it: Perception against Stealth for Hidden, Insight or Investigation for Disguised, Investigation for Illusory, Perception or Investigation for Secret.
- A Reveal Effect removes the qualities it names inside its area for everyone, whatever their Senses.
- Truesight does not beat Stealth or find secret objects; it sees invisible and ethereal creatures, true forms and through illusions.
