ALTER TABLE users
  DROP COLUMN IF EXISTS password_hash,
  DROP COLUMN IF EXISTS auth_provider,
  DROP COLUMN IF EXISTS google_id;
