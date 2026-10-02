-- +goose Up
SET lock_timeout = '5s';
SET statement_timeout = '60s';

-- A named group of an account's Library entries, such as a homebrew expansion.
CREATE TABLE IF NOT EXISTS library.collections (
    id uuid PRIMARY KEY,
    owner_subject text NOT NULL CHECK (length(owner_subject) BETWEEN 1 AND 255),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    description text NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS collections_owner_idx ON library.collections (owner_subject, name);

CREATE TABLE IF NOT EXISTS library.collection_entries (
    collection_id uuid NOT NULL REFERENCES library.collections (id) ON DELETE CASCADE,
    entry_id uuid NOT NULL REFERENCES library.entries (id) ON DELETE CASCADE,
    PRIMARY KEY (collection_id, entry_id)
);
CREATE INDEX IF NOT EXISTS collection_entries_entry_idx ON library.collection_entries (entry_id);

-- A Collection switched on in a Campaign: its entries show there while the row exists.
CREATE TABLE IF NOT EXISTS library.campaign_collections (
    campaign_id uuid NOT NULL REFERENCES campaign.campaigns (id) ON DELETE CASCADE,
    collection_id uuid NOT NULL REFERENCES library.collections (id) ON DELETE CASCADE,
    switched_at timestamptz NOT NULL,
    PRIMARY KEY (campaign_id, collection_id)
);
CREATE INDEX IF NOT EXISTS campaign_collections_collection_idx ON library.campaign_collections (collection_id);

-- A link a DM made themselves, as opposed to one that only holds the Campaign Override or pin of an
-- entry a Collection brings in.
ALTER TABLE library.campaign_links ADD COLUMN IF NOT EXISTS direct boolean NOT NULL DEFAULT true;
