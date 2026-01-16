ALTER TABLE IF EXISTS expert_availabilities
DROP CONSTRAINT IF EXISTS unique_expert_day;

DROP INDEX IF EXISTS idx_expert_day;
 