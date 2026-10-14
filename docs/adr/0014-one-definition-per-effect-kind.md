# One definition per Effect kind, and timestamped migrations that may run out of order

Every SRD spell, Feature, monster action and item is moving to full automation, written by many
sessions in parallel. Adding an Effect kind used to edit every exhaustive switch over kinds in five
files, redefine one shared check constraint, and take the next migration number, so two sessions
adding kinds always collided.

Each kind now lives in files of its own: a Go file in `internal/rules/effects` with its type, its
definition (the fields an author fills in and the checks on them), its rules text and its
capabilities in play; a store in `internal/compendium/pgstore` with its own query file; and a
migration that creates its table and adds its row to `compendium.effect_kinds`, the lookup table that
replaced the shared check. A kind is registered by one sorted line in a list of kinds in each layer:
the linter bans package globals, so kinds cannot register themselves, and the pure rules package
cannot name its store. A test fails when the two lists disagree.

In play, a passive kind folds into the profiles attacks and saves read through small capability
interfaces it chooses to implement, and a kind that acts when its Effect lands returns acts from a
small closed set the runtime applies. A switch over the older kinds reaches every kind defined this
way through one `case Part:`.

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
list of kinds. Kinds still on the older path keep working beside it until they move. Because
migrations can now apply out of order, a migration must not depend on another migration merged in the
same window; a kind's migration touches only its own table and its own row.
