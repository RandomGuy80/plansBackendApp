-- Users
CREATE TABLE IF NOT EXISTS users (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone            TEXT UNIQUE NOT NULL,
  username         TEXT,
  telegram_chat_id BIGINT DEFAULT 0,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Meetings
CREATE TABLE IF NOT EXISTS meetings (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  category    TEXT NOT NULL DEFAULT '',
  place       TEXT NOT NULL,
  lat         DOUBLE PRECISION NOT NULL DEFAULT 57.1522,
  lng         DOUBLE PRECISION NOT NULL DEFAULT 65.5272,
  when_ts     TIMESTAMPTZ NOT NULL,
  max_people  INT NOT NULL DEFAULT 3,
  joined      INT NOT NULL DEFAULT 1,
  desc        TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Meeting members (for join/leave)
CREATE TABLE IF NOT EXISTS meeting_members (
  meeting_id UUID NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (meeting_id, user_id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS meetings_when_ts_idx ON meetings(when_ts);
CREATE INDEX IF NOT EXISTS meetings_host_id_idx ON meetings(host_id);
