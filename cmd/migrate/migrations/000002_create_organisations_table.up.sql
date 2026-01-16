CREATE TABLE IF NOT EXISTS organisations (
    id SERIAL PRIMARY KEY UNIQUE NOT NULL, 
    org_name TEXT NOT NULL UNIQUE,
    about_org TEXT NOT NULL,
    owner_id INT NOT NULL, 
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 0,
    FOREIGN KEY (owner_id) REFERENCES users(id)
);   

   