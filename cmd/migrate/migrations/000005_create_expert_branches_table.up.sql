CREATE TABLE IF NOT EXISTS expert_branches(
    id SERIAL PRIMARY KEY,
    expert_id INT NOT NULL,
    branch_id INT NOT NULL,
    UNIQUE (expert_id, branch_id), -- Prevents duplicate expert-branch pairs
    FOREIGN KEY (expert_id) REFERENCES experts(id) ON DELETE CASCADE,
    FOREIGN KEY (branch_id) REFERENCES branches(id) ON DELETE CASCADE
);    