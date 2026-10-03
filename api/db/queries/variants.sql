-- name: ListRuleVariants :many
SELECT variant, value FROM campaign.rule_variants WHERE campaign_id = $1 ORDER BY variant;

-- name: SetRuleVariant :exec
INSERT INTO campaign.rule_variants (campaign_id, variant, value, updated_at)
VALUES (@campaign_id, @variant, @value, @now)
ON CONFLICT (campaign_id, variant) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;

-- name: CampaignShortRests :one
SELECT short_rests FROM campaign.campaigns WHERE id = $1;

-- name: SetCampaignShortRests :exec
UPDATE campaign.campaigns SET short_rests = @short_rests WHERE id = @id;
