CREATE TABLE IF NOT EXISTS event_db.event_fencing_tokens (
    event_id BIGINT UNSIGNED PRIMARY KEY,
    last_issued_token BIGINT UNSIGNED NOT NULL DEFAULT 0,
    last_accepted_token BIGINT UNSIGNED NOT NULL DEFAULT 0
) ENGINE=InnoDB;
