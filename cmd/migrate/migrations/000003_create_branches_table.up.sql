CREATE TABLE IF NOT EXISTS branches (
    id SERIAL PRIMARY KEY UNIQUE NOT NULL,
    branch_name TEXT NOT NULL,
    organisation_id INT NOT NULL, 
    about_branch TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 0,
    FOREIGN KEY (organisation_id) REFERENCES organisations(id)
);
  