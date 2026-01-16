-- Drop the indexes first
DROP INDEX IF EXISTS idx_zoom_meeting_participants_meeting_id;
DROP INDEX IF EXISTS idx_zoom_meeting_participants_meeting_uuid;
DROP INDEX IF EXISTS idx_zoom_meeting_participants_participant_id;

-- Drop the columns
ALTER TABLE zoom_meeting_participants
DROP COLUMN IF EXISTS meeting_id,
DROP COLUMN IF EXISTS meeting_uuid,
DROP COLUMN IF EXISTS participant_id,
DROP COLUMN IF EXISTS participant_name,
DROP COLUMN IF EXISTS participant_email,
DROP COLUMN IF EXISTS duration_seconds,
DROP COLUMN IF EXISTS created_at,
DROP COLUMN IF EXISTS updated_at;