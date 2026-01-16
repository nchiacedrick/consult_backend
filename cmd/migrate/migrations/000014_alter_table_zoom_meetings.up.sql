ALTER TABLE zoom_meetings
ADD COLUMN IF NOT EXISTS slot_id INT REFERENCES timeslots(id);