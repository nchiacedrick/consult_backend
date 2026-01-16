ALTER TABLE zoom_meeting_participants
ADD COLUMN IF NOT EXISTS zoom_meeting_id VARCHAR(255);