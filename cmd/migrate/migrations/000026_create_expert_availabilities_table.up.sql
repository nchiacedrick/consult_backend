CREATE TABLE IF NOT EXISTS expert_availabilities (
    id SERIAL PRIMARY KEY,
    expert_id INT NOT NULL REFERENCES experts(id) ON DELETE CASCADE,
    day_of_week VARCHAR(10) CHECK (day_of_week IN 
        ('monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday')),
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    is_weekend BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT valid_time_range CHECK (start_time < end_time)
);
