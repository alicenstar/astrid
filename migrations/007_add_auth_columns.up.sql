ALTER TABLE users
  ADD COLUMN password_hash TEXT,
  ADD COLUMN auth_provider TEXT NOT NULL DEFAULT 'local',
  ADD COLUMN google_id TEXT UNIQUE;
