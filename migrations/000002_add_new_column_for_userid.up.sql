ALTER TABLE urls ADD COLUMN user_id VARCHAR(255);

CREATE INDEX idx_user_id ON urls(user_id);