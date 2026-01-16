CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS timeslots(
    id SERIAL UNIQUE NOT NULL, 
    host_id INT NOT NULL, 
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time   TIMESTAMP WITH TIME ZONE NOT NULL,
    is_booked BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 0,
    FOREIGN KEY (host_id) REFERENCES experts(id),
    CONSTRAINT start_timestamp_not_future CHECK (start_time >= CURRENT_TIMESTAMP), 
    CONSTRAINT end_timestamp_not_future CHECK (end_time > CURRENT_TIMESTAMP),
    CONSTRAINT end_time_after_start_time CHECK (end_time > start_time),
    CONSTRAINT no_overlapping_timeslots EXCLUDE USING GIST (
        host_id WITH =,
        tstzrange(start_time, end_time) WITH &&
    )
);              