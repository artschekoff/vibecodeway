-- +goose Up
CREATE TABLE IF NOT EXISTS app_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    level      TEXT    NOT NULL,
    ts         REAL    NOT NULL,
    caller     TEXT    NOT NULL DEFAULT '',
    msg        TEXT    NOT NULL,
    error      TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS app_logs;
