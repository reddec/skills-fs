-- +migrate Up

-- A skill is an Agent Skills directory: its `name` is both the directory name and the
-- required SKILL.md frontmatter `name`. Go validates the spec rules; the CHECKs below are
-- defense-in-depth (SQLite has no REGEXP, so GLOB enforces allowed chars + hyphen rules).
CREATE TABLE skill (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT NOT NULL UNIQUE,
    description   TEXT NOT NULL,
    body          TEXT NOT NULL DEFAULT '',
    license       TEXT,
    compatibility TEXT,
    allowed_tools TEXT,
    metadata      TEXT NOT NULL DEFAULT '{}',
    created_at    TIMESTAMP NOT NULL DEFAULT current_timestamp,
    updated_at    TIMESTAMP NOT NULL DEFAULT current_timestamp,
    CHECK (length(name) BETWEEN 1 AND 64),
    CHECK (name NOT GLOB '*[^a-z0-9-]*'),
    CHECK (name NOT GLOB '-*' AND name NOT GLOB '*-' AND name NOT GLOB '*--*')
);

CREATE TABLE token (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    label        TEXT NOT NULL DEFAULT '',
    token_hash   TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL DEFAULT '',
    last_used_at TIMESTAMP,
    created_at   TIMESTAMP NOT NULL DEFAULT current_timestamp
);

-- +migrate Down

DROP TABLE token;
DROP TABLE skill;
