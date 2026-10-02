-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Every Campaign Character gets its own Container, and the armor, shield and weapons its build names as
-- Item Instances: armor worn, the shield in the off hand, the first weapon in the main hand.
INSERT INTO campaign.containers (id, campaign_id, kind, character_id, label, created_at)
SELECT gen_random_uuid(), c.campaign_id, 'character', c.id, c.name, now()
FROM campaign.characters c
WHERE NOT EXISTS (SELECT 1 FROM campaign.containers k WHERE k.character_id = c.id);

INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, equipped_slot, created_at)
SELECT gen_random_uuid(), k.id, c.armor_slug, 1, true, false, 'armor', now()
FROM campaign.characters c JOIN campaign.containers k ON k.character_id = c.id
WHERE c.armor_slug IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM campaign.item_instances i WHERE i.container_id = k.id AND i.equipped_slot = 'armor');

INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, equipped_slot, created_at)
SELECT gen_random_uuid(), k.id, 'shield', 1, true, false, 'off_hand', now()
FROM campaign.characters c JOIN campaign.containers k ON k.character_id = c.id
WHERE c.shield
    AND NOT EXISTS (SELECT 1 FROM campaign.item_instances i WHERE i.container_id = k.id AND i.equipped_slot = 'off_hand');

INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, equipped_slot, created_at)
SELECT gen_random_uuid(), k.id, w.weapon_slug, 1, true, false,
    CASE WHEN w.ordering = 0 AND NOT EXISTS (SELECT 1 FROM campaign.item_instances m WHERE m.container_id = k.id AND m.equipped_slot = 'main_hand')
        THEN 'main_hand' END,
    now()
FROM campaign.character_weapons w JOIN campaign.containers k ON k.character_id = w.character_id
WHERE NOT EXISTS (SELECT 1 FROM campaign.item_instances i WHERE i.container_id = k.id AND i.item_slug = w.weapon_slug);
