CREATE TABLE users (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  name            TEXT NOT NULL UNIQUE,
  email           TEXT,
  public_key      TEXT NOT NULL,
  private_key     TEXT NOT NULL,
  preshared_key   TEXT NOT NULL,
  ip_address      TEXT NOT NULL UNIQUE,
  endpoint        TEXT,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

CREATE TABLE user_groups (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  name            TEXT NOT NULL UNIQUE,
  lan_mode        INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

CREATE TABLE user_group_members (
  user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  group_id        INTEGER NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, group_id)
);

CREATE TABLE services (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  name            TEXT NOT NULL UNIQUE,
  hostname        TEXT NOT NULL UNIQUE,
  origin          TEXT NOT NULL,
  ip_address      TEXT NOT NULL UNIQUE,
  description     TEXT,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

CREATE TABLE service_ports (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  service_id      INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  port            INTEGER NOT NULL,
  protocol        TEXT NOT NULL CHECK (protocol IN ('tcp','udp')),
  UNIQUE (service_id, port, protocol)
);

CREATE TABLE service_groups (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  name            TEXT NOT NULL UNIQUE,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

CREATE TABLE service_group_members (
  service_id      INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  group_id        INTEGER NOT NULL REFERENCES service_groups(id) ON DELETE CASCADE,
  PRIMARY KEY (service_id, group_id)
);

CREATE TABLE policy_rules (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  subject_type    TEXT NOT NULL CHECK (subject_type IN ('user','user_group')),
  subject_id      INTEGER NOT NULL,
  object_type     TEXT NOT NULL CHECK (object_type IN ('user','user_group','service','service_group')),
  object_id       INTEGER NOT NULL,
  action          TEXT NOT NULL CHECK (action IN ('allow','deny')),
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);

CREATE TABLE settings (
  key             TEXT PRIMARY KEY,
  value           TEXT NOT NULL
);

CREATE INDEX idx_policy_rules_subject ON policy_rules (subject_type, subject_id);
CREATE INDEX idx_policy_rules_object ON policy_rules (object_type, object_id);
CREATE INDEX idx_user_group_members_group ON user_group_members (group_id);
CREATE INDEX idx_service_group_members_group ON service_group_members (group_id);
