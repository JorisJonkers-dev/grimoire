-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A Container holds at most one plain stack of each item: no name, Charges, slot or Attunement, and
-- identified. Anything else is singled out as its own Item Instance.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX IF NOT EXISTS item_instances_stack_idx ON campaign.item_instances (container_id, item_slug)
    WHERE custom_name IS NULL AND charges IS NULL AND equipped_slot IS NULL AND NOT attuned AND identified;

-- Every slug-keyed stack becomes a plain stack Instance; the old rows stay until nothing reads them.
INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
SELECT gen_random_uuid(), i.container_id, i.item_slug, i.quantity, true, false, c.created_at
FROM campaign.container_items i JOIN campaign.containers c ON c.id = i.container_id
ON CONFLICT (container_id, item_slug) WHERE custom_name IS NULL AND charges IS NULL AND equipped_slot IS NULL AND NOT attuned AND identified
DO NOTHING;
