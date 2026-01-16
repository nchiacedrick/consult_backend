DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'participant_role') THEN
        CREATE TYPE participant_role AS ENUM ('host', 'co-host', 'participant');
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS zoom_meeting_participants (
    id SERIAL PRIMARY KEY,
    meeting_id INT NOT NULL REFERENCES zoom_meetings(id),
    user_id INT NOT NULL REFERENCES users(id),  
    role participant_role NOT NULL DEFAULT 'participant',
    UNIQUE(meeting_id, user_id),
    joined_at TIMESTAMP,
    left_at TIMESTAMP  
);       

