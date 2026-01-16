CREATE TABLE IF NOT EXISTS chats (
    id SERIAL PRIMARY KEY NOT NULL UNIQUE,
    sender_id INT NOT NULL,
    reciever_id INT NOT NULL,
    content TEXT NOT NULL, 
    picture TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (sender_id) REFERENCES users(id),
    FOREIGN KEY (reciever_id) REFERENCES users(id)
);