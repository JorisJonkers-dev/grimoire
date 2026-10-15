# One definition per Effect kind, and timestamped migrations that may run out of order

Every SRD spell, Feature, monster action and item is moving to full automation, written by many
sessions in parallel. Adding an Effect kind used to edit every exhaustive switch over kinds in five
files, redefine one shared check constraint, and take the next migration number, so two sessions
adding kinds always collided.

Each kind now lives in files of its own: a Go file in `internal/rules/effects` with its type, its
definition (the fields an author fills in and the checks on them), its rules text and its
capabilities in play; a store in `internal/compendium/pgstore` with its own query file; and a
migration that creates its table and adds its row to `compendium.effect_kinds`, the lookup table that
replaced the shared check. A Part beneath another names its parent and the group it sits in on its
own row. A kind is registered by one sorted line in a list of kinds in each layer: the linter bans
package globals, so kinds cannot register themselves, and the pure rules package cannot name its
store. A test fails when the two lists disagree.

In play, a kind acts only through small capability interfaces it chooses to implement. A passive
kind folds into the profile that reads it: the attacks its bearer makes (`AttackFolder`) or suffers
(`AttackedFolder`), its saves (`SaveFolder`), its movement (`MoveFolder` into a `MoveProfile`) and
what it can do on a turn (`ActionFolder` into an `ActionProfile`); a `Stacker` makes its Effect stack
in levels, with a cap and a level that kills. A kind that acts when its Effect lands returns acts from
a small closed set the runtime applies: into an area's plan (`Asker`), on each creature in the area
once the rolls are in (`Lander`), or on its bearer (`BearerLander`). A kind that holds Parts beneath
it does so in named groups (`Grouper`): a branch brings its group into play as the Effect lands
(`Holder`), and a choice offers its groups as modes (`Chooser`). A kind marks the fields that grow
with the level its Effect is activated at (`Scaler`), and its Part grows them (`Grower`), so Scaling
reaches any kind's field without naming the kind. Every kind is defined this way, so no switch names a
kind.

Migrations take timestamp versions (`yyyymmddhhmmss_name.sql`) from here on, and goose applies a
lower version that arrives after a higher one. The numbered migrations already applied keep their
numbers.

## Considered options

- A code generator writing the switches from a list of kinds: keeps the switches, but every kind
  still edits the generated files and the list, so sessions still collide.
- One generic table of kind, field and value rows: no per-kind table, but no constraint can check a
  field, and it is JSONB in all but name.
- Kinds registering themselves in `init`: one file per kind and no shared line, but it needs a package
  global, which the linter bans and which hides the list from a reader.

## Consequences

A new kind adds its rules file, its store file, its query file, its migration and one line in each
list of kinds, and touches no switch. The API carries every Part in one generic recursive shape,
checked by the kind's own definition, and serves each kind's definition, so a new kind needs no change
to the OpenAPI document either. Because migrations can now apply out of order, a migration must
not depend on another migration merged in the same window; a kind's migration touches only its own
table and its own row.
