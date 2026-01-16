ALTER TABLE expert_branches
ADD COLUMN IF NOT EXISTS req_status VARCHAR(20)
    CHECK (req_status IN ('requested', 'accepted', 'waiting'))
    DEFAULT 'waiting';