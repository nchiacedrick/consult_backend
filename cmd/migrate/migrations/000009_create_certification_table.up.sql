CREATE TABLE IF NOT EXISTS certifications (
    id SERIAl UNIQUE NOT NULL,
    picture TEXT NOT NULL,
    cert_name TEXT NOT NULL,
    institution TEXT NOT NULL, 
    expert_id INT NOT NULL, 
    cert_date TIMESTAMP WITH TIME ZONE NOT NULL,
    CONSTRAINT cert_timestamp_not_future CHECK (cert_date <= CURRENT_TIMESTAMP),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);