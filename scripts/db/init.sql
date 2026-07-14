CREATE DATABASE IF NOT EXISTS user_db;
CREATE DATABASE IF NOT EXISTS event_db;
CREATE DATABASE IF NOT EXISTS ticket_db;
CREATE DATABASE IF NOT EXISTS email_db;

USE user_db;
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name_enc VARBINARY(512) NOT NULL,
    email_enc VARBINARY(512) NOT NULL,
    email_hash VARCHAR(64) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_created_at (created_at),
    INDEX idx_email_hash (email_hash)
);

USE event_db;
CREATE TABLE IF NOT EXISTS events (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    date DATETIME NOT NULL,
    venue VARCHAR(255) NOT NULL,
    total_capacity INT UNSIGNED NOT NULL,
    remaining_count INT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_date (date),
    INDEX idx_remaining_count (remaining_count),
    CONSTRAINT chk_remaining CHECK (remaining_count >= 0 AND remaining_count <= total_capacity)
);

USE ticket_db;
CREATE TABLE IF NOT EXISTS tickets (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    booking_ref CHAR(12) NOT NULL UNIQUE,
    user_id BIGINT UNSIGNED NOT NULL,
    event_id BIGINT UNSIGNED NOT NULL,
    quantity INT UNSIGNED NOT NULL,
    status ENUM('confirmed', 'cancelled') NOT NULL DEFAULT 'confirmed',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_event_id (event_id),
    INDEX idx_booking_ref (booking_ref),
    INDEX idx_created_at (created_at),
    CONSTRAINT chk_quantity CHECK (quantity > 0)
);

USE email_db;
CREATE TABLE IF NOT EXISTS email_status (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    booking_ref CHAR(12) NOT NULL DEFAULT '',
    ticket_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    recipient_hash VARCHAR(64) NOT NULL,
    status ENUM('pending', 'sent', 'failed', 'dead') NOT NULL DEFAULT 'pending',
    retry_count INT UNSIGNED NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_booking_ref (booking_ref),
    INDEX idx_ticket_id (ticket_id),
    INDEX idx_status (status),
    INDEX idx_last_attempt (last_attempt_at)
);
