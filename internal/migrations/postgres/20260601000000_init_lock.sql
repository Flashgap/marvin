-- Create "lock_users" table
CREATE TABLE lock_users (
    slack_user_id   VARCHAR(64) PRIMARY KEY,
    slack_user_name VARCHAR(255) NOT NULL DEFAULT '',
    points          INTEGER NOT NULL DEFAULT 0,
    updated_at      TIMESTAMP NOT NULL
);

-- Create index "lock_users_points_idx" to "lock_users" table
CREATE INDEX lock_users_points_idx ON lock_users (points);

-- Create "lock_events" table
CREATE TABLE lock_events (
    victim_id  VARCHAR(64) NOT NULL,
    finder_id  VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (victim_id, finder_id, created_at)
);
