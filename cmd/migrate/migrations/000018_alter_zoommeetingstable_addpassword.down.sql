DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'zoom_meetings' AND column_name = 'password'
    ) THEN
        ALTER TABLE zoom_meetings
        DROP COLUMN password;
    END IF;
END $$;