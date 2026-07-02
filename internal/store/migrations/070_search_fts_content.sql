-- +goose Up

-- Recreate the FTS index with an item_id column so matches can be traced back
-- to the concrete element (issue, thread, text, file, branch) they came from.
-- The index is repopulated from source tables on the next search via
-- RebuildSearchIndex, so no data migration is needed here.
DROP TABLE IF EXISTS context_search_fts;

CREATE VIRTUAL TABLE context_search_fts USING fts5(
    context_id UNINDEXED,
    item_id UNINDEXED,
    source UNINDEXED,
    body,
    tokenize = 'unicode61 remove_diacritics 2'
);

-- +goose Down

DROP TABLE IF EXISTS context_search_fts;

CREATE VIRTUAL TABLE context_search_fts USING fts5(
    context_id UNINDEXED,
    source UNINDEXED,
    body,
    tokenize = 'unicode61 remove_diacritics 2'
);
