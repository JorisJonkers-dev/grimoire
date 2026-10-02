-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Container can sit inside another, as a bag in a Character's Inventory. Each has exactly one owner:
-- a Character, the Campaign (the Party Stash and drops of loot), or the Container it sits in.
ALTER TABLE campaign.containers ADD COLUMN IF NOT EXISTS parent_id uuid;
ALTER TABLE campaign.containers ADD CONSTRAINT containers_parent_fk FOREIGN KEY (parent_id)
    REFERENCES campaign.containers (id) ON DELETE CASCADE NOT VALID;
ALTER TABLE campaign.containers DROP CONSTRAINT IF EXISTS containers_kind_check;
ALTER TABLE campaign.containers ADD CONSTRAINT containers_kind_check
    CHECK (kind IN ('character', 'party_stash', 'loot_drop', 'bag')) NOT VALID;
ALTER TABLE campaign.containers ADD CONSTRAINT containers_one_owner_check
    CHECK ((kind = 'bag') = (parent_id IS NOT NULL) AND parent_id IS DISTINCT FROM id) NOT VALID;

-- One particular item: a base or homebrew item by slug, with its own name, Charges, and whether it is
-- identified, attuned and worn. Plain stackable gear keeps a quantity instead; anything singled out by
-- a name, Charges, a slot or Attunement is one item.
CREATE TABLE IF NOT EXISTS campaign.item_instances (
    id uuid PRIMARY KEY,
    container_id uuid NOT NULL REFERENCES campaign.containers (id) ON DELETE CASCADE,
    item_slug text NOT NULL CHECK (char_length(item_slug) BETWEEN 1 AND 80),
    custom_name text CHECK (char_length(custom_name) BETWEEN 1 AND 80),
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 100000),
    charges integer CHECK (charges BETWEEN 0 AND 100),
    identified boolean NOT NULL,
    attuned boolean NOT NULL,
    equipped_slot text CHECK (equipped_slot IN ('main_hand', 'off_hand', 'ranged_main', 'ranged_off', 'armor', 'head', 'cloak',
        'hands', 'feet', 'neck', 'ring_1', 'ring_2')),
    created_at timestamptz NOT NULL,
    CONSTRAINT item_instances_single_check
        CHECK (quantity = 1 OR (custom_name IS NULL AND charges IS NULL AND equipped_slot IS NULL AND NOT attuned)),
    CONSTRAINT item_instances_attuned_check CHECK (identified OR NOT attuned)
);

-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS item_instances_container_idx ON campaign.item_instances (container_id, created_at);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS item_instances_slot_idx ON campaign.item_instances (container_id, equipped_slot)
    WHERE equipped_slot IS NOT NULL;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX IF NOT EXISTS containers_parent_idx ON campaign.containers (parent_id) WHERE parent_id IS NOT NULL;
