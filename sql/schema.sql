PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY NOT NULL,
    instruction_text TEXT NOT NULL,
    status INTEGER NOT NULL,
    result_text TEXT,
    created_at INTEGER DEFAULT (unixepoch()),
    started_at INTEGER,
    finished_at INTEGER
);


CREATE TABLE IF NOT EXISTS issues(
    issue_key TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    loe INTEGER,
    started_at INTEGER,
    finished_at INTEGER,
    team_id INTEGER REFERENCES teams(id) NOT NULL,
    updated_at INTEGER DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS teams(
    id INTEGER PRIMARY KEY,
    label TEXT NOT NULL UNIQUE
);
INSERT OR IGNORE INTO teams (id, label) VALUES (467, 'API');
INSERT OR IGNORE INTO teams (id, label) VALUES (6988, 'APR');
INSERT OR IGNORE INTO teams (id, label) VALUES (478, 'APIENG');


CREATE TABLE IF NOT EXISTS team_aliases(
    alias TEXT,
    team_id INTEGER REFERENCES teams(id),
    PRIMARY KEY (alias, team_id)
);
INSERT OR IGNORE INTO team_aliases (alias, team_id) VALUES ('api dev moscow', 467);
INSERT OR IGNORE INTO team_aliases (alias, team_id) VALUES ('api dev remote', 6988);
INSERT OR IGNORE INTO team_aliases (alias, team_id) VALUES ('api dev india', 478);
