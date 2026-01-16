DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'meeting_status'
    ) THEN
        CREATE TYPE meeting_status AS ENUM (
            'scheduled', 'started', 'ended', 'waiting',
            'canceled', 'deleted', 'recurring', 'expired'
        );
    END IF;
END
$$;

CREATE  TABLE IF NOT EXISTS zoom_meetings (
    id SERIAL PRIMARY KEY,
    zoom_meeting_id BIGINT NOT NULL UNIQUE,  -- Zoom's internal ID
    meeting_url TEXT NOT NULL, 
    created_by INT NOT NULL REFERENCES users(id),   -- User who created the meeting
    zoom_host_id TEXT,
    zoom_host_email TEXT,                     -- Optional: Zoom's host ID (if different from created_by)
    topic VARCHAR(255),
    start_time TIMESTAMP NOT NULL,
    duration INT NOT NULL,                           -- Minutes
    timezone VARCHAR(50) DEFAULT 'UTC',
    agenda TEXT,  
    zoom_status meeting_status DEFAULT 'scheduled',       -- 'scheduled'/'started'/'ended'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);              

         