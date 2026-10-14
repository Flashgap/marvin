-- Create "standup_users" table
CREATE TABLE standup_users (
    slack_user_id    VARCHAR(64) PRIMARY KEY,
    -- Slack message ts, e.g. "1759820000.123456": a message ID, not a time.
    last_update_ts   VARCHAR(32) NULL,
    last_update_text TEXT NULL,
    last_reminded_at TIMESTAMP NULL,
    updated_at       TIMESTAMP NOT NULL
);
