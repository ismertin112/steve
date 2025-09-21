CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE,
    username TEXT,
    key_id TEXT,
    expires_at TIMESTAMP,
    status TEXT DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS payments (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    screenshot_url TEXT,
    status TEXT DEFAULT 'pending',
    comment TEXT,
    created_at TIMESTAMP DEFAULT now()
);
