-- Remove unique constraint on (meeting_id, user_id)
ALTER TABLE IF EXISTS zoom_meeting_participants
DROP CONSTRAINT IF EXISTS zoom_meeting_participants_meeting_id_user_id_key;