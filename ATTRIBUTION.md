# Attribution

Grimoire's code is licensed under the [Attribution Assurance License](LICENSE). This file lists
third-party content and software Grimoire uses, with the attribution each license requires. It is
part of the Definition of Done: adding SRD content, imported data or embedded third-party code adds
an entry here.

## Game content

| Source | License | Required attribution |
|---|---|---|
| System Reference Document 5.2 | CC-BY-4.0 | "This work includes material from the System Reference Document 5.2 ("SRD 5.2") by Wizards of the Coast LLC, available at https://www.dndbeyond.com/srd. The SRD 5.2 is licensed under the Creative Commons Attribution 4.0 International License, available at https://creativecommons.org/licenses/by/4.0/legalcode." |
| System Reference Document 5.1 | CC-BY-4.0 | "This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 International License available at https://creativecommons.org/licenses/by/4.0/legalcode." |

## Import sources

| Snapshot | Source | Taken | Contents |
|---|---|---|---|
| `api/db/seeds/compendium.json.gz` | [Open5e](https://open5e.com) v2 API (`srd-2024`, `srd-2014`) | 2026-09-29 | 658 spells, 30 condition texts, 48 classes and subclasses, 22 species, 5 backgrounds, 18 feats, 75 weapons, 25 armor, 1,699 items and magic items, 656 monsters |

The snapshot holds only SRD material; Open5e's own code is not included. Refresh it with
`go run ./cmd/grimoire snapshot` from `api/`, then compare it with [5e-bits](https://www.dnd5eapi.co)
using `go run ./cmd/grimoire crosscheck`, which rewrites [docs/compendium-crosscheck.md](docs/compendium-crosscheck.md).

## Third-party software

Recorded here as dependencies are added. Only permissive licenses may be embedded; see
[ADR-0002](docs/adr/0002-attribution-assurance-license.md).
