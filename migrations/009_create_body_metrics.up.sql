CREATE TABLE body_metrics (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date         DATE NOT NULL,
    weight_kg    NUMERIC NOT NULL,
    body_fat_pct NUMERIC,
    muscle_pct   NUMERIC,
    notes        TEXT DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, date)
);
