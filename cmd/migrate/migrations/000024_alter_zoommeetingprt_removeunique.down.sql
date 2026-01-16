ALTER TABLE IF EXISTS zoom_meeting_participants
ADD CONSTRAINT unique_meeting_user UNIQUE (meeting_id, user_id);