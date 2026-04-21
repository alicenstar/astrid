CREATE TABLE user_profiles (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    height_cm  NUMERIC,
    birth_date DATE,
    sex        TEXT CHECK (sex IN ('male', 'female')),
    activity_level TEXT CHECK (activity_level IN ('sedentary', 'light', 'moderate', 'active', 'very_active')),
    weight_unit TEXT NOT NULL DEFAULT 'lbs' CHECK (weight_unit IN ('kg', 'lbs')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
