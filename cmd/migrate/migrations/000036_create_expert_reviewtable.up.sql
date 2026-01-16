CREATE TABLE IF NOT EXISTS expert_reviews (
    id SERIAL PRIMARY KEY,
    expert_id INT NOT NULL REFERENCES experts(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    review TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (expert_id, user_id)
);
   