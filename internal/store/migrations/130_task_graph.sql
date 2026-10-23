-- +goose Up

-- Provider-neutral task graph cache (Jira today, Linear later). Nodes are my
-- open issues plus whatever they point at (parents up to the epic, blockers,
-- related issues, subtasks). Edges are re-derived per node from its `owner`'s
-- detail fetch, so a refresh of one node replaces only the edges it produced.
CREATE TABLE IF NOT EXISTS task_nodes (
    provider      TEXT NOT NULL,
    key           TEXT NOT NULL,
    summary       TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT '',
    issue_type    TEXT NOT NULL DEFAULT '',
    priority      TEXT NOT NULL DEFAULT '',
    priority_rank INTEGER NOT NULL DEFAULT 0,
    assignee      TEXT NOT NULL DEFAULT '',
    parent_key    TEXT NOT NULL DEFAULT '',
    mine          INTEGER NOT NULL DEFAULT 0,
    terminal      INTEGER NOT NULL DEFAULT 0,
    fetched_at    TEXT NOT NULL DEFAULT '',
    detail_at     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (provider, key)
);

CREATE TABLE IF NOT EXISTS task_edges (
    provider TEXT NOT NULL,
    owner    TEXT NOT NULL,
    src      TEXT NOT NULL,
    dst      TEXT NOT NULL,
    kind     TEXT NOT NULL, -- parent | blocks | relates
    PRIMARY KEY (provider, owner, src, dst, kind)
);
CREATE INDEX IF NOT EXISTS idx_task_edges_src ON task_edges(provider, src);
CREATE INDEX IF NOT EXISTS idx_task_edges_dst ON task_edges(provider, dst);

-- +goose Down
DROP TABLE task_edges;
DROP TABLE task_nodes;
