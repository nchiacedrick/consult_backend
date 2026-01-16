-- ==========================================================
-- Migration: Add advanced booking constraints and triggers
-- Description:
--   - Prevent overlapping bookings for same expert or user
--   - Prevent past bookings
--   - Prevent bookings outside expert availability
--   - Enforce minimum 30-minute duration
-- ==========================================================

-- Ensure required extension for exclusion constraints
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ==========================================================
-- 1️⃣ Add computed time range column (for exclusion constraints)
-- ==========================================================
ALTER TABLE IF EXISTS bookings
ADD COLUMN IF NOT EXISTS time_range tstzrange
    GENERATED ALWAYS AS (tstzrange(start_time, end_time, '[)')) STORED;

-- ==========================================================
-- 2️⃣ Prevent expert double-booking
-- ==========================================================
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'no_expert_overlap'
    ) THEN
        ALTER TABLE bookings
        ADD CONSTRAINT no_expert_overlap
        EXCLUDE USING gist (
            expert_id WITH =,
            time_range WITH &&
        )
        WHERE (bk_status IN ('pending', 'confirmed'));
    END IF;
END$$;

-- ==========================================================
-- 3️⃣ Prevent user double-booking
-- ==========================================================
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'no_user_overlap'
    ) THEN
        ALTER TABLE bookings
        ADD CONSTRAINT no_user_overlap
        EXCLUDE USING gist (
            user_id WITH =,
            time_range WITH &&
        )
        WHERE (bk_status IN ('pending', 'confirmed'));
    END IF;
END$$;

-- ==========================================================
-- 4️⃣ Create trigger function to enforce booking rules
-- ==========================================================
CREATE OR REPLACE FUNCTION enforce_booking_rules()
RETURNS TRIGGER AS $$
DECLARE
    v_available_start TIME;
    v_available_end TIME;
    v_day TEXT;
BEGIN
     ------------------------------------------------------------------
    -- 0️⃣ Prevent expert from booking himself
    ------------------------------------------------------------------
    IF NEW.user_id = (SELECT user_id FROM experts WHERE id = NEW.expert_id) THEN
    RAISE EXCEPTION
        'An expert cannot book himself. The user (ID: %) is the same as the expert’s user (ID: %).',
        NEW.user_id, (SELECT user_id FROM experts WHERE id = NEW.expert_id);
    END IF;
    ------------------------------------------------------------------
    -- Prevent booking in the past
    ------------------------------------------------------------------
    IF NEW.start_time < NOW() THEN
        RAISE EXCEPTION 'Cannot book a session in the past.';
    END IF;

    ------------------------------------------------------------------
    -- Prevent end_time before start_time
    ------------------------------------------------------------------
    IF NEW.end_time <= NEW.start_time THEN
        RAISE EXCEPTION 'End time must be after start time.';
    END IF;

    ------------------------------------------------------------------
    -- Enforce minimum booking duration (≥ 30 minutes)
    ------------------------------------------------------------------
    IF (NEW.end_time - NEW.start_time) < INTERVAL '30 minutes' THEN
        RAISE EXCEPTION 'Booking duration must be at least 30 minutes.';
    END IF;

    ------------------------------------------------------------------
    -- Ensure booking fits within expert availability hours
    ------------------------------------------------------------------
    SELECT ea.start_time, ea.end_time
    INTO v_available_start, v_available_end
    FROM expert_availabilities ea
    WHERE ea.expert_id = NEW.expert_id
      AND TRIM(LOWER(ea.day_of_week)) =
          TRIM(LOWER(TO_CHAR(NEW.start_time AT TIME ZONE 'UTC', 'FMday')))
      AND (NEW.start_time::TIME >= ea.start_time AND NEW.end_time::TIME <= ea.end_time)
    LIMIT 1;

    IF v_available_start IS NULL THEN
        SELECT TRIM(LOWER(TO_CHAR(NEW.start_time AT TIME ZONE 'UTC', 'FMday')))
        INTO v_day;

        RAISE EXCEPTION
            'Booking time (%, %) is outside expert available hours for %. Expert availability not found (Expert ID: %)',
            NEW.start_time::time,
            NEW.end_time::time,
            v_day,
            NEW.expert_id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ==========================================================
-- 5️⃣ Attach trigger to enforce validation on insert/update
-- ==========================================================
DROP TRIGGER IF EXISTS validate_booking_time ON bookings;

CREATE TRIGGER validate_booking_time
BEFORE INSERT OR UPDATE ON bookings
FOR EACH ROW
EXECUTE FUNCTION enforce_booking_rules();

-- ==========================================================
-- ✅ Migration complete
-- ==========================================================
