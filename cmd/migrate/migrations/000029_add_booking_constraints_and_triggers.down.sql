-- ==========================================================
-- Rollback Migration: Remove booking constraints and triggers
-- ==========================================================

-- Remove trigger and function
DROP TRIGGER IF EXISTS validate_booking_time ON bookings;
DROP FUNCTION IF EXISTS enforce_booking_rules();

-- Drop exclusion constraints
ALTER TABLE IF EXISTS bookings
DROP CONSTRAINT IF EXISTS no_expert_overlap,
DROP CONSTRAINT IF EXISTS no_user_overlap;

-- Drop generated range column
ALTER TABLE IF EXISTS bookings
DROP COLUMN IF EXISTS time_range;

-- (Extension remains since it may be used elsewhere)
