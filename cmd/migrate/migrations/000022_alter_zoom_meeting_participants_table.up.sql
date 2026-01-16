ALTER TABLE IF EXISTS zoom_meeting_participants
ADD COLUMN IF NOT EXISTS meeting_uuid VARCHAR(255),
ADD COLUMN IF NOT EXISTS participant_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS participant_name VARCHAR(255),  
ADD COLUMN IF NOT EXISTS participant_email VARCHAR(255),
ADD COLUMN IF NOT EXISTS duration_seconds INTEGER,
ADD COLUMN IF NOT EXISTS meeting_id BIGINT,
ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
  
-- Add indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_zoom_meeting_participants_meeting_id ON zoom_meeting_participants(meeting_id);
CREATE INDEX IF NOT EXISTS idx_zoom_meeting_participants_meeting_uuid ON zoom_meeting_participants(meeting_uuid);
CREATE INDEX IF NOT EXISTS idx_zoom_meeting_participants_participant_id ON zoom_meeting_participants(participant_id);