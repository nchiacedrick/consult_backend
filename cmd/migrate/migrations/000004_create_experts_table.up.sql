   
CREATE TABLE IF NOT EXISTS experts(
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL UNIQUE,
    expertise TEXT NOT NULL,
    bio TEXT NOT NULL, 
    rating NUMERIC(3,2) DEFAULT 0,
    fees_per_hr NUMERIC(10,2) NOT NULL,
    verified BOOLEAN DEFAULT FALSE, 
    version INT NOT NULL DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

   