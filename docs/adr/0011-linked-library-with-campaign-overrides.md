# Library entries are linked into Campaigns, with Campaign Overrides

A DM's Library entries (creatures, NPCs, Homebrew, prep tables, Locations, Maps) are referenced from a Campaign rather than copied into it. Campaign-specific state (hit points, location, attitude, death, notes) is stored as a Campaign Override on top of the shared base, so fixing a stat block or lore once fixes it everywhere. Revisions record changes to the base and to each override separately. Copy-on-use was rejected because it silently forks content across Campaigns. The Shared Library and Collections use the same linking.

## Consequences

Editing a base entry changes every Campaign that uses it. The editor must show where an entry is used, and a DM who wants a variant duplicates it explicitly.
