CREATE TABLE daily_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, date)
);

CREATE TABLE meals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    daily_log_id UUID NOT NULL REFERENCES daily_logs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    calories INTEGER NOT NULL CHECK (calories >= 0),
    protein_g NUMERIC(6,1) NOT NULL DEFAULT 0,
    fiber_g NUMERIC(6,1) NOT NULL DEFAULT 0,
    cholesterol_mg NUMERIC(6,1) NOT NULL DEFAULT 0,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
