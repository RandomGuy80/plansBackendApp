-- Run this migration if telegram_chat_id column doesn't exist yet
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS telegram_chat_id BIGINT;
