-- ==========================================
-- 🧹 Down Migration: Clean invalid bookings and drop objects
-- ==========================================

DO $$
BEGIN
    -- Check if the bookings table exists before cleaning it
    IF EXISTS (
        SELECT 1 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
          AND table_name = 'bookings'
    ) THEN
        RAISE NOTICE 'Cleaning invalid bookings (NULL or invalid time ranges)...';
        DELETE FROM bookings
        WHERE start_time IS NULL
           OR end_time IS NULL
           OR start_time >= end_time;
    ELSE
        RAISE NOTICE 'Bookings table not found, skipping cleanup.';
    END IF;
END $$;

-- ==========================================================
--  Drop dependent database objects safely
-- ==========================================================

-- First drop triggers and functions to avoid dependency errors
DROP TRIGGER IF EXISTS validate_booking_time ON bookings;
DROP FUNCTION IF EXISTS enforce_booking_rules();

-- Drop exclusion constraints safely
ALTER TABLE IF EXISTS bookings DROP CONSTRAINT IF EXISTS no_user_overlap;
ALTER TABLE IF EXISTS bookings DROP CONSTRAINT IF EXISTS no_expert_overlap;

-- Finally drop the main table and types
DROP TABLE IF EXISTS bookings CASCADE;
DROP TYPE IF EXISTS payment_state CASCADE;

RAISE NOTICE '✅ Down migration completed successfully.';
