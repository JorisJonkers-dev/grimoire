-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- Grimoire plays SRD 5.2 only. Campaigns move to the 2024 rules; the 5.1 documents stay readable in the
-- compendium.
UPDATE campaign.campaigns SET ruleset_pref = 'srd-2024' WHERE ruleset_pref <> 'srd-2024';

-- Characters move with their Campaign when their class, species and background exist in the 2024
-- rules. A build that has no 2024 counterpart (a 5.1-only species, say) keeps its ruleset so it still
-- reads correctly; its owner rebuilds it.
UPDATE campaign.characters ch
SET ruleset = 'srd-2024'
WHERE ch.ruleset <> 'srd-2024'
  AND EXISTS (
    SELECT 1 FROM compendium.classes c JOIN compendium.documents d ON d.id = c.document_id
    WHERE d.key = 'srd-2024' AND c.slug = ch.class_slug)
  AND EXISTS (
    SELECT 1 FROM compendium.species s JOIN compendium.documents d ON d.id = s.document_id
    WHERE d.key = 'srd-2024' AND s.slug = ch.species_slug)
  AND EXISTS (
    SELECT 1 FROM compendium.backgrounds b JOIN compendium.documents d ON d.id = b.document_id
    WHERE d.key = 'srd-2024' AND b.slug = ch.background_slug);

ALTER TABLE campaign.campaigns DROP CONSTRAINT IF EXISTS campaigns_ruleset_pref_check;
ALTER TABLE campaign.campaigns ADD CONSTRAINT campaigns_ruleset_pref_check CHECK (ruleset_pref = 'srd-2024') NOT VALID;
