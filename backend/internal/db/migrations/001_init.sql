CREATE TABLE topics (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  name          TEXT NOT NULL,
  query         TEXT NOT NULL,
  include_json  TEXT NOT NULL DEFAULT '[]',
  exclude_json  TEXT NOT NULL DEFAULT '[]',
  understood    TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL CHECK (status IN ('trial', 'following', 'dropped')),
  created_at    TEXT NOT NULL,
  last_run_at   TEXT
);

CREATE TABLE items (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  topic_id      INTEGER NOT NULL REFERENCES topics(id),
  url           TEXT NOT NULL,
  title         TEXT NOT NULL,
  source        TEXT NOT NULL DEFAULT '',
  published_at  TEXT,
  summary       TEXT,
  reason        TEXT NOT NULL DEFAULT '',
  run_date      TEXT NOT NULL,
  created_at    TEXT NOT NULL,
  UNIQUE (topic_id, url)
);
CREATE INDEX items_run_date ON items(run_date);

CREATE TABLE runs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  topic_id    INTEGER NOT NULL REFERENCES topics(id),
  started_at  TEXT NOT NULL,
  ok          INTEGER NOT NULL DEFAULT 0,
  error       TEXT NOT NULL DEFAULT '',
  n_results   INTEGER NOT NULL DEFAULT 0,
  n_kept      INTEGER NOT NULL DEFAULT 0
);
