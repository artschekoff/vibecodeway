-- +goose Up

CREATE VIRTUAL TABLE IF NOT EXISTS context_search_fts USING fts5(
    context_id UNINDEXED,
    source UNINDEXED,
    body,
    tokenize = 'unicode61 remove_diacritics 2'
);

-- +goose Down

DROP TABLE IF EXISTS context_search_fts;
