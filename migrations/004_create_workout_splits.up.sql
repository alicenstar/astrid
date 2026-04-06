CREATE TABLE workout_splits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE split_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_split_id UUID NOT NULL REFERENCES workout_splits(id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    label TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    UNIQUE(workout_split_id, day_of_week)
);

CREATE TABLE planned_exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    split_day_id UUID NOT NULL REFERENCES split_days(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sets INTEGER NOT NULL DEFAULT 0,
    reps INTEGER NOT NULL DEFAULT 0,
    notes TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0
);
