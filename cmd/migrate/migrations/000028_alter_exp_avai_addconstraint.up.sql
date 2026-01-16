-- Use a unique index instead of ADD CONSTRAINT IF NOT EXISTS (not supported by PostgreSQL)
CREATE UNIQUE INDEX IF NOT EXISTS idx_expert_day_unique
ON expert_availabilities (expert_id, day_of_week);