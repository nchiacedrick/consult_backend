-- 1️⃣ Add missing columns (one per statement)
ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS start_time TIMESTAMP WITH TIME ZONE;

ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS end_time TIMESTAMP WITH TIME ZONE;

-- add the FK column first (add constraint separately so migration is idempotent/safe)
ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS expert_id INT;

-- add status column with default; constraint added below
ALTER TABLE IF EXISTS bookings
    ADD COLUMN IF NOT EXISTS bk_status VARCHAR(20) DEFAULT 'pending';

-- 2️⃣ Allow nulls on existing columns safely and add constraints (only if table/columns exist)
DO $$
BEGIN
    -- allow NULLs safely
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'bookings' AND column_name = 'slot_id'
    ) THEN
        EXECUTE 'ALTER TABLE bookings ALTER COLUMN slot_id DROP NOT NULL';
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'bookings' AND column_name = 'zoom_meeting_id'
    ) THEN
        EXECUTE 'ALTER TABLE bookings ALTER COLUMN zoom_meeting_id DROP NOT NULL';
    END IF;

    -- only proceed if bookings table exists
    IF EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relname = 'bookings' AND n.nspname = current_schema()
    ) THEN

        -- add foreign key constraint for expert_id if experts table exists and constraint not present
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema() AND table_name = 'bookings' AND column_name = 'expert_id'
        )
        AND EXISTS (
            SELECT 1 FROM pg_class c2
            JOIN pg_namespace n2 ON n2.oid = c2.relnamespace
            WHERE c2.relname = 'experts' AND n2.nspname = current_schema()
        ) THEN
            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint
                WHERE conrelid = 'bookings'::regclass AND conname = 'bookings_expert_id_fkey'
            ) THEN
                EXECUTE 'ALTER TABLE bookings ADD CONSTRAINT bookings_expert_id_fkey FOREIGN KEY (expert_id) REFERENCES experts(id) ON DELETE CASCADE';
            END IF;
        END IF;

        -- add bk_status check constraint if not present
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema() AND table_name = 'bookings' AND column_name = 'bk_status'
        ) THEN
            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint
                WHERE conrelid = 'bookings'::regclass AND conname = 'bookings_bk_status_check'
            ) THEN
                EXECUTE $sql$
                    ALTER TABLE bookings
                    ADD CONSTRAINT bookings_bk_status_check
                    CHECK (bk_status IN ('pending', 'confirmed', 'cancelled', 'completed'));
                $sql$;
            END IF;
        END IF;

        -- add time-related check constraints only if both columns exist
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema() AND table_name = 'bookings' AND column_name = 'start_time'
        )
        AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = current_schema() AND table_name = 'bookings' AND column_name = 'end_time'
        ) THEN

            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint
                WHERE conrelid = 'bookings'::regclass AND conname = 'valid_booking_time'
            ) THEN
                EXECUTE 'ALTER TABLE bookings ADD CONSTRAINT valid_booking_time CHECK (end_time > start_time)';
            END IF;

            IF NOT EXISTS (
                SELECT 1 FROM pg_constraint
                WHERE conrelid = 'bookings'::regclass AND conname = 'minimum_duration'
            ) THEN
                EXECUTE 'ALTER TABLE bookings ADD CONSTRAINT minimum_duration CHECK (end_time >= start_time + INTERVAL ''30 minutes'')';
            END IF;
        END IF;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_bookings_expert_time ON bookings(expert_id, start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_availabilities_expert_day ON expert_availabilities(expert_id, day_of_week);

CREATE UNIQUE INDEX IF NOT EXISTS unique_expert_booking_time
ON bookings (expert_id, start_time, end_time)
WHERE bk_status = 'confirmed';
