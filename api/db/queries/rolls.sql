-- name: LockCampaignLog :exec
SELECT pg_advisory_xact_lock(hashtextextended(@lock_key::text, 0));

-- name: NextActionSeq :one
SELECT coalesce(max(seq), 0)::bigint + 1 FROM play.actions WHERE campaign_id = $1;

-- name: InsertAction :one
INSERT INTO play.actions (campaign_id, seq, kind, actor_member_id, actor_name, origin, client, seed, created_at)
VALUES (@campaign_id, @seq, @kind, @actor_member_id, @actor_name, @origin, @client, sqlc.narg(seed), @now)
RETURNING id;

-- name: InsertRollEvent :exec
INSERT INTO play.action_roll_events (action_id, roll_id, die_no, value) VALUES (@action_id, @roll_id, sqlc.narg(die_no), @value);

-- name: InsertRoll :one
INSERT INTO play.roll_requests (id, campaign_id, purpose, notation, requested_by_name, roller_member_id, roller_subject,
    roller_name, status, created_at, asked)
VALUES (COALESCE(sqlc.narg(id)::uuid, gen_random_uuid()), @campaign_id, @purpose, @notation, @requested_by_name,
    @roller_member_id, @roller_subject, @roller_name, 'pending', @now, @asked)
RETURNING id;

-- name: InsertRollLabel :exec
INSERT INTO play.roll_request_labels (roll_id, group_no, label) VALUES (@roll_id, @group_no, @label);

-- name: InsertRollModifier :exec
INSERT INTO play.roll_request_modifiers (roll_id, ordering, label, value) VALUES (@roll_id, @ordering, @label, @value);

-- name: InsertRollDie :exec
INSERT INTO play.roll_dice (roll_id, die_no, group_no, faces) VALUES (@roll_id, @die_no, @group_no, @faces);

-- name: GetRoll :one
SELECT id, campaign_id, purpose, notation, requested_by_name, roller_member_id, roller_subject, roller_name, status, total,
       created_at, resolved_at, choosing, rerolled, asked
FROM play.roll_requests WHERE campaign_id = @campaign_id AND id = @id;

-- name: LockRoll :one
SELECT status FROM play.roll_requests WHERE campaign_id = @campaign_id AND id = @id FOR UPDATE;

-- name: ListRolls :many
SELECT id FROM play.roll_requests WHERE campaign_id = @campaign_id ORDER BY created_at DESC, id DESC LIMIT @page_size;

-- name: RollLabels :many
SELECT group_no, label FROM play.roll_request_labels WHERE roll_id = $1 ORDER BY group_no;

-- name: RollModifiers :many
SELECT label, value FROM play.roll_request_modifiers WHERE roll_id = $1 ORDER BY ordering;

-- name: RollDice :many
SELECT die_no, group_no, faces, value, mode, karmic_dropped FROM play.roll_dice WHERE roll_id = $1 ORDER BY die_no;

-- name: SetRollDie :execrows
UPDATE play.roll_dice SET value = @value, mode = @mode, karmic_dropped = sqlc.narg(karmic_dropped) WHERE roll_id = @roll_id AND die_no = @die_no AND value IS NULL;

-- name: ResolveRoll :exec
UPDATE play.roll_requests SET status = 'resolved', total = @total, resolved_at = @now WHERE id = @id;

-- name: ActionLog :many
SELECT a.seq, a.kind, a.actor_name, a.origin, a.client, a.seed, a.created_at, e.roll_id, e.die_no, e.value
FROM play.actions a LEFT JOIN play.action_roll_events e ON e.action_id = a.id
WHERE a.campaign_id = @campaign_id
ORDER BY a.seq DESC LIMIT @page_size;

-- name: SetRollChoice :exec
UPDATE play.roll_requests SET choosing = @choosing, rerolled = rerolled OR @rerolled WHERE id = @id;

-- name: MemberInspired :one
-- Whether a member plays a Character in the Campaign that holds Heroic Inspiration.
SELECT EXISTS (SELECT 1 FROM campaign.characters WHERE campaign_id = @campaign_id AND owner_member_id = @member_id AND heroic_inspiration)::boolean;

-- name: SpendMemberInspiration :execrows
UPDATE campaign.characters SET heroic_inspiration = false
WHERE id = (SELECT id FROM campaign.characters c WHERE c.campaign_id = @campaign_id AND c.owner_member_id = @member_id AND c.heroic_inspiration ORDER BY c.id LIMIT 1);

-- name: RerollDie :execrows
UPDATE play.roll_dice SET value = @value, mode = 'auto', karmic_dropped = NULL WHERE roll_id = @roll_id AND die_no = @die_no AND value IS NOT NULL;

-- name: CampaignKarmicDice :one
SELECT karmic_dice FROM campaign.campaigns WHERE id = $1;

-- name: RollKarma :one
SELECT recent FROM play.roll_karma WHERE campaign_id = @campaign_id AND member_id = @member_id FOR UPDATE;

-- name: SetRollKarma :exec
INSERT INTO play.roll_karma (campaign_id, member_id, recent) VALUES (@campaign_id, @member_id, @recent)
ON CONFLICT (campaign_id, member_id) DO UPDATE SET recent = EXCLUDED.recent;
