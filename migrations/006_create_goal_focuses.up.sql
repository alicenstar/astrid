CREATE TABLE goal_focuses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nutrient_type TEXT NOT NULL,
    daily_target NUMERIC(8,1) NOT NULL CHECK (daily_target > 0),
    unit TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
