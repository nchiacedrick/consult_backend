DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'zoom_meetings' AND column_name = 'password'
    ) THEN
        ALTER TABLE zoom_meetings
        ADD COLUMN password VARCHAR(50);
    END IF;
END $$;