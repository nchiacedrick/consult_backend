DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_state') THEN
        CREATE TYPE payment_state AS ENUM ('pending', 'success', 'refunded', 'cancelled', 'failed');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS bookings(
    id SERIAL PRIMARY KEY NOT NULL,  
    user_id INT NOT NULL,
    slot_id INT NOT NULL, 
    zoom_meeting_id INT NOT NULL,  -- Strong link to Zoom meeting
    payment_status payment_state NOT NULL DEFAULT 'pending', 
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 0,
    UNIQUE(user_id, slot_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (zoom_meeting_id) REFERENCES zoom_meetings(id),  
    FOREIGN KEY (slot_id) REFERENCES timeslots(id)
);                 