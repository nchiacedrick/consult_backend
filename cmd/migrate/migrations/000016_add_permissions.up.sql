CREATE TABLE IF NOT EXISTS permissions (
    id bigserial PRIMARY KEY,
    code text NOT NULL
);

CREATE TABLE IF NOT EXISTS users_permissions (
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id bigint NOT NULL REFERENCES permissions ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);

-- Add the two permissions to the table.
INSERT INTO permissions (code)
VALUES
('organisations:read'),
('organisations:write'),
('branches:read'),
('branches:write'),
('experts:read'),
('experts:write'),
('timeslots:read'),
('timeslots:write'),
('bookings:read'),
('bookings:write'),
('users:read'),
('users:write'),
('permissions:read'),
('permissions:write');  