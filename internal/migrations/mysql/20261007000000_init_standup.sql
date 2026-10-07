-- Create "standup_users" table
CREATE TABLE standup_users (
    slack_user_id    VARCHAR(64) NOT NULL,
    last_update_ts   VARCHAR(32) NULL,
    last_update_text MEDIUMTEXT CHARACTER SET utf8mb4 NULL,
    last_reminded_at DATETIME NULL,
    updated_at       DATETIME NOT NULL,
    PRIMARY KEY (slack_user_id)
);
