-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A creature on the map can belong to a Faction: its Standing shapes social checks with it.
ALTER TABLE play.tokens ADD COLUMN IF NOT EXISTS faction_id uuid;
ALTER TABLE play.tokens DROP CONSTRAINT IF EXISTS tokens_faction_fk;
ALTER TABLE play.tokens ADD CONSTRAINT tokens_faction_fk FOREIGN KEY (faction_id) REFERENCES campaign.factions (id) ON DELETE SET NULL NOT VALID;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS tokens_faction_idx ON play.tokens (faction_id) WHERE faction_id IS NOT NULL;

-- A Shop can belong to a Faction: it prices by how the Faction regards whoever buys.
ALTER TABLE prep.shops ADD COLUMN IF NOT EXISTS faction_id uuid;
ALTER TABLE prep.shops DROP CONSTRAINT IF EXISTS shops_faction_fk;
ALTER TABLE prep.shops ADD CONSTRAINT shops_faction_fk FOREIGN KEY (faction_id) REFERENCES campaign.factions (id) ON DELETE SET NULL NOT VALID;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS shops_faction_idx ON prep.shops (faction_id) WHERE faction_id IS NOT NULL;
ALTER TABLE prep.shop_revisions ADD COLUMN IF NOT EXISTS faction_id uuid;

-- An entry of an Encounter Table can be a Faction's own: it weighs by how the Faction regards the party.
ALTER TABLE prep.table_entries ADD COLUMN IF NOT EXISTS faction_id uuid;
ALTER TABLE prep.table_entries DROP CONSTRAINT IF EXISTS table_entries_faction_fk;
ALTER TABLE prep.table_entries ADD CONSTRAINT table_entries_faction_fk FOREIGN KEY (faction_id) REFERENCES campaign.factions (id) ON DELETE SET NULL NOT VALID;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS table_entries_faction_idx ON prep.table_entries (faction_id) WHERE faction_id IS NOT NULL;
ALTER TABLE prep.table_revision_entries ADD COLUMN IF NOT EXISTS faction_id uuid;
