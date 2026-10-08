-- Create "standup_users" table
CREATE TABLE standup_users (
    slack_user_id    VARCHAR(64) PRIMARY KEY,
    last_update_ts   VARCHAR(32) NULL,
    last_update_text TEXT NULL,
    last_reminded_at TIMESTAMP NULL,
    updated_at       TIMESTAMP NOT NULL
);
