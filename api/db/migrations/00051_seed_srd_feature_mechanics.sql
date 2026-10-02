-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- The SRD 5.2 Resources, scaling values, choices and prerequisites. Rows already present are left alone,
-- so a homebrew edit to a seeded row survives a re-run.
-- +goose StatementBegin
DO $$
DECLARE r bigint;
BEGIN
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('rage', 'Rage', 'class', 'barbarian', 'table', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 1, 2);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 3, 3);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 6, 4);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 12, 5);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 17, 6);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 1, 1, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('bardic-inspiration', 'Bardic Inspiration', 'class', 'bard', 'ability', 0, 'charisma', 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_dice (resource_id, level, die) VALUES (r, 1, 'd6');
        INSERT INTO compendium.resource_dice (resource_id, level, die) VALUES (r, 5, 'd8');
        INSERT INTO compendium.resource_dice (resource_id, level, die) VALUES (r, 10, 'd10');
        INSERT INTO compendium.resource_dice (resource_id, level, die) VALUES (r, 15, 'd12');
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 5, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('channel-divinity', 'Channel Divinity', 'class', 'cleric', 'table', 0, NULL, 2) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 2, 2);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 6, 3);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 18, 4);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 2, 1, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 2, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('wild-shape', 'Wild Shape', 'class', 'druid', 'table', 0, NULL, 2) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 2, 2);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 6, 3);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 17, 4);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 2, 1, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 2, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('second-wind', 'Second Wind', 'class', 'fighter', 'table', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 1, 2);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 4, 3);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 10, 4);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 1, 1, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('action-surge', 'Action Surge', 'class', 'fighter', 'table', 0, NULL, 2) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 2, 1);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 17, 2);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 2, NULL, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 2, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('indomitable', 'Indomitable', 'class', 'fighter', 'table', 0, NULL, 9) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 9, 1);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 13, 2);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 17, 3);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 9, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('focus-points', 'Focus Points', 'class', 'monk', 'class_level', 1, NULL, 2) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 2, NULL, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 2, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('lay-on-hands', 'Lay On Hands', 'class', 'paladin', 'class_level', 5, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('paladin-channel-divinity', 'Channel Divinity', 'class', 'paladin', 'table', 0, NULL, 3) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 3, 2);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 11, 3);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 3, 1, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 3, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('favored-enemy', 'Favored Enemy', 'class', 'ranger', 'table', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 1, 2);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 5, 3);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 9, 4);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 13, 5);
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 17, 6);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('sorcery-points', 'Sorcery Points', 'class', 'sorcerer', 'class_level', 1, NULL, 2) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 2, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('innate-sorcery', 'Innate Sorcery', 'class', 'sorcerer', 'table', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 1, 2);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('magical-cunning', 'Magical Cunning', 'class', 'warlock', 'table', 0, NULL, 2) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 2, 1);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 2, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('arcane-recovery', 'Arcane Recovery', 'class', 'wizard', 'table', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 1, 1);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('breath-weapon', 'Breath Weapon', 'species', 'dragonborn', 'proficiency', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('stonecunning', 'Stonecunning', 'species', 'dwarf', 'proficiency', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('large-form', 'Large Form', 'species', 'goliath', 'table', 0, NULL, 5) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 5, 1);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 5, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('adrenaline-rush', 'Adrenaline Rush', 'species', 'orc', 'proficiency', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'short_rest', 1, NULL, NULL);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.resources (slug, name, owner_kind, owner_slug, basis, multiplier, ability, from_level) VALUES ('relentless-endurance', 'Relentless Endurance', 'species', 'orc', 'table', 0, NULL, 1) ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.resource_maxima (resource_id, level, maximum) VALUES (r, 1, 1);
        INSERT INTO compendium.resource_recharges (resource_id, event, from_level, amount, roll_at_least) VALUES (r, 'long_rest', 1, NULL, NULL);
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('sneak-attack', 'Sneak Attack', 'class', 'rogue') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, '1d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 3, '2d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 5, '3d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 7, '4d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 9, '5d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 11, '6d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 13, '7d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 15, '8d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 17, '9d6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 19, '10d6');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('martial-arts', 'Martial Arts', 'class', 'monk') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, 'd6');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 5, 'd8');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 11, 'd10');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 17, 'd12');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('rage-damage', 'Rage Damage', 'class', 'barbarian') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, '+2');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 9, '+3');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 16, '+4');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('unarmored-movement', 'Unarmored Movement', 'class', 'monk') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 2, '+10 ft');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 6, '+15 ft');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 10, '+20 ft');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 14, '+25 ft');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 18, '+30 ft');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('barbarian-weapon-mastery', 'Weapon Mastery', 'class', 'barbarian') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, '2');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 4, '3');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 10, '4');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('fighter-weapon-mastery', 'Weapon Mastery', 'class', 'fighter') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, '3');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 4, '4');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 10, '5');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 16, '6');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('paladin-weapon-mastery', 'Weapon Mastery', 'class', 'paladin') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, '2');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('ranger-weapon-mastery', 'Weapon Mastery', 'class', 'ranger') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, '2');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('rogue-weapon-mastery', 'Weapon Mastery', 'class', 'rogue') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 1, '2');
    END IF;
    INSERT INTO compendium.scales (slug, name, owner_kind, owner_slug) VALUES ('brutal-strike', 'Brutal Strike', 'class', 'barbarian') ON CONFLICT (slug) DO NOTHING RETURNING id INTO r;
    IF r IS NOT NULL THEN
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 9, '1d10');
        INSERT INTO compendium.scale_steps (scale_id, level, value) VALUES (r, 17, '2d10');
    END IF;
END
$$;
-- +goose StatementEnd

INSERT INTO compendium.choices (owner_kind, owner_slug, slug, name, level, count, pool, pool_from) VALUES
    ('class', 'barbarian', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'barbarian'),
    ('class', 'barbarian', 'subclass', 'Subclass', 3, 1, 'subclass', 'barbarian'),
    ('class', 'barbarian', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'barbarian', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'barbarian', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'barbarian', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'barbarian', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'barbarian', 'weapon-mastery', 'Weapon Mastery', 1, 2, 'weapon', 'any'),
    ('class', 'bard', 'skills', 'Skill proficiencies', 1, 3, 'skill', 'bard'),
    ('class', 'bard', 'subclass', 'Subclass', 3, 1, 'subclass', 'bard'),
    ('class', 'bard', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'bard', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'bard', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'bard', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'bard', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'cleric', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'cleric'),
    ('class', 'cleric', 'subclass', 'Subclass', 3, 1, 'subclass', 'cleric'),
    ('class', 'cleric', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'cleric', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'cleric', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'cleric', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'cleric', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'druid', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'druid'),
    ('class', 'druid', 'subclass', 'Subclass', 3, 1, 'subclass', 'druid'),
    ('class', 'druid', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'druid', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'druid', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'druid', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'druid', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'fighter', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'fighter'),
    ('class', 'fighter', 'subclass', 'Subclass', 3, 1, 'subclass', 'fighter'),
    ('class', 'fighter', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'fighter', 'feat', 'Ability Score Improvement or feat', 6, 1, 'feat_category', 'general'),
    ('class', 'fighter', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'fighter', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'fighter', 'feat', 'Ability Score Improvement or feat', 14, 1, 'feat_category', 'general'),
    ('class', 'fighter', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'fighter', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'fighter', 'weapon-mastery', 'Weapon Mastery', 1, 3, 'weapon', 'any'),
    ('class', 'monk', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'monk'),
    ('class', 'monk', 'subclass', 'Subclass', 3, 1, 'subclass', 'monk'),
    ('class', 'monk', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'monk', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'monk', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'monk', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'monk', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'paladin', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'paladin'),
    ('class', 'paladin', 'subclass', 'Subclass', 3, 1, 'subclass', 'paladin'),
    ('class', 'paladin', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'paladin', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'paladin', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'paladin', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'paladin', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'paladin', 'weapon-mastery', 'Weapon Mastery', 1, 2, 'weapon', 'any'),
    ('class', 'ranger', 'skills', 'Skill proficiencies', 1, 3, 'skill', 'ranger'),
    ('class', 'ranger', 'subclass', 'Subclass', 3, 1, 'subclass', 'ranger'),
    ('class', 'ranger', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'ranger', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'ranger', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'ranger', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'ranger', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'ranger', 'weapon-mastery', 'Weapon Mastery', 1, 2, 'weapon', 'any'),
    ('class', 'rogue', 'skills', 'Skill proficiencies', 1, 4, 'skill', 'rogue'),
    ('class', 'rogue', 'subclass', 'Subclass', 3, 1, 'subclass', 'rogue'),
    ('class', 'rogue', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'rogue', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'rogue', 'feat', 'Ability Score Improvement or feat', 10, 1, 'feat_category', 'general'),
    ('class', 'rogue', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'rogue', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'rogue', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'rogue', 'weapon-mastery', 'Weapon Mastery', 1, 2, 'weapon', 'any'),
    ('class', 'sorcerer', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'sorcerer'),
    ('class', 'sorcerer', 'subclass', 'Subclass', 3, 1, 'subclass', 'sorcerer'),
    ('class', 'sorcerer', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'sorcerer', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'sorcerer', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'sorcerer', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'sorcerer', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'warlock', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'warlock'),
    ('class', 'warlock', 'subclass', 'Subclass', 3, 1, 'subclass', 'warlock'),
    ('class', 'warlock', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'warlock', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'warlock', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'warlock', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'warlock', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'wizard', 'skills', 'Skill proficiencies', 1, 2, 'skill', 'wizard'),
    ('class', 'wizard', 'subclass', 'Subclass', 3, 1, 'subclass', 'wizard'),
    ('class', 'wizard', 'feat', 'Ability Score Improvement or feat', 4, 1, 'feat_category', 'general'),
    ('class', 'wizard', 'feat', 'Ability Score Improvement or feat', 8, 1, 'feat_category', 'general'),
    ('class', 'wizard', 'feat', 'Ability Score Improvement or feat', 12, 1, 'feat_category', 'general'),
    ('class', 'wizard', 'feat', 'Ability Score Improvement or feat', 16, 1, 'feat_category', 'general'),
    ('class', 'wizard', 'epic-boon', 'Epic Boon', 19, 1, 'feat_category', 'epic-boon'),
    ('class', 'fighter', 'fighting-style', 'Fighting Style', 1, 1, 'feat_category', 'fighting-style'),
    ('class', 'paladin', 'fighting-style', 'Fighting Style', 2, 1, 'feat_category', 'fighting-style'),
    ('class', 'ranger', 'fighting-style', 'Fighting Style', 2, 1, 'feat_category', 'fighting-style'),
    ('class', 'rogue', 'expertise', 'Expertise', 1, 2, 'expertise', 'rogue'),
    ('class', 'rogue', 'expertise', 'Expertise', 6, 2, 'expertise', 'rogue'),
    ('class', 'bard', 'expertise', 'Expertise', 2, 2, 'expertise', 'bard'),
    ('class', 'bard', 'expertise', 'Expertise', 9, 2, 'expertise', 'bard'),
    ('class', 'ranger', 'expertise', 'Expertise', 2, 1, 'expertise', 'ranger'),
    ('class', 'wizard', 'expertise', 'Expertise', 2, 1, 'expertise', 'wizard'),
    ('species', 'human', 'versatile', 'Versatile', 1, 1, 'feat_category', 'origin'),
    ('species', 'human', 'skillful', 'Skillful', 1, 1, 'skill', 'any'),
    ('species', 'elf', 'elven-lineage', 'Elven Lineage', 1, 1, 'listed', 'drow|high-elf|wood-elf'),
    ('species', 'dragonborn', 'draconic-ancestry', 'Draconic Ancestry', 1, 1, 'listed', 'black|blue|brass|bronze|copper|gold|green|red|silver|white'),
    ('species', 'tiefling', 'fiendish-legacy', 'Fiendish Legacy', 1, 1, 'listed', 'abyssal|chthonic|infernal'),
    ('species', 'gnome', 'gnomish-lineage', 'Gnomish Lineage', 1, 1, 'listed', 'forest-gnome|rock-gnome'),
    ('species', 'goliath', 'giant-ancestry', 'Giant Ancestry', 1, 1, 'listed', 'cloud|fire|frost|hill|stone|storm'),
    ('feat', 'magic-initiate', 'spell-list', 'Spell list', 1, 1, 'listed', 'cleric|druid|wizard'),
    ('feat', 'skilled', 'skills', 'Skills or tools', 1, 3, 'skill', 'any'),
    ('background', 'acolyte', 'ability-scores', 'Ability score increases', 1, 1, 'listed', '+2/+1|+1/+1/+1'),
    ('background', 'criminal', 'ability-scores', 'Ability score increases', 1, 1, 'listed', '+2/+1|+1/+1/+1'),
    ('background', 'sage', 'ability-scores', 'Ability score increases', 1, 1, 'listed', '+2/+1|+1/+1/+1'),
    ('background', 'soldier', 'ability-scores', 'Ability score increases', 1, 1, 'listed', '+2/+1|+1/+1/+1')
ON CONFLICT DO NOTHING;

INSERT INTO compendium.prerequisites (owner_kind, owner_slug, group_no, ordinal, kind, ability, minimum, ref_slug) VALUES
    ('feat', 'boon-of-combat-prowess', 0, 0, 'level', NULL, 19, NULL),
    ('feat', 'boon-of-dimensional-travel', 0, 0, 'level', NULL, 19, NULL),
    ('feat', 'boon-of-fate', 0, 0, 'level', NULL, 19, NULL),
    ('feat', 'boon-of-irresistible-offense', 0, 0, 'level', NULL, 19, NULL),
    ('feat', 'boon-of-spell-recall', 0, 0, 'level', NULL, 19, NULL),
    ('feat', 'boon-of-the-night-spirit', 0, 0, 'level', NULL, 19, NULL),
    ('feat', 'boon-of-truesight', 0, 0, 'level', NULL, 19, NULL),
    ('feat', 'boon-of-spell-recall', 1, 0, 'spellcasting', NULL, 0, NULL),
    ('feat', 'archery', 0, 0, 'feature', NULL, 0, 'fighting-style'),
    ('feat', 'defense', 0, 0, 'feature', NULL, 0, 'fighting-style'),
    ('feat', 'great-weapon-fighting', 0, 0, 'feature', NULL, 0, 'fighting-style'),
    ('feat', 'two-weapon-fighting', 0, 0, 'feature', NULL, 0, 'fighting-style'),
    ('feat', 'ability-score-improvement', 0, 0, 'level', NULL, 4, NULL),
    ('feat', 'grappler', 0, 0, 'level', NULL, 4, NULL),
    ('feat', 'grappler', 1, 0, 'ability', 'strength', 13, NULL),
    ('feat', 'grappler', 1, 1, 'ability', 'dexterity', 13, NULL)
ON CONFLICT DO NOTHING;
