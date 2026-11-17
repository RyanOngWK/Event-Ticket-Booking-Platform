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
    price_cents BIGINT UNSIGNED NOT NULL DEFAULT 2500,
    currency CHAR(3) NOT NULL DEFAULT 'usd',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_date (date),
    INDEX idx_remaining_count (remaining_count),
    CONSTRAINT chk_remaining CHECK (remaining_count >= 0 AND remaining_count <= total_capacity)
);

-- Inventory is represented by one row per saleable unit. The purchase path
-- claims these rows with SELECT ... FOR UPDATE SKIP LOCKED instead of
-- serializing every buyer on events.remaining_count.
CREATE TABLE IF NOT EXISTS reservation_units (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_id BIGINT UNSIGNED NOT NULL,
    unit_number INT UNSIGNED NOT NULL,
    status ENUM('available', 'reserved', 'sold') NOT NULL DEFAULT 'available',
    reservation_id CHAR(36) NULL,
    ticket_id BIGINT UNSIGNED NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_event_unit (event_id, unit_number),
    INDEX idx_available_units (event_id, status, id)
) ENGINE=InnoDB;

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

CREATE TABLE IF NOT EXISTS reservations (
    id CHAR(36) PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    event_id BIGINT UNSIGNED NOT NULL,
    quantity INT UNSIGNED NOT NULL,
    amount_cents BIGINT UNSIGNED NOT NULL,
    currency CHAR(3) NOT NULL,
    status ENUM('pending_payment', 'purchased', 'expired', 'payment_setup_failed', 'cancelled') NOT NULL DEFAULT 'pending_payment',
    payment_intent_id VARCHAR(255) NULL UNIQUE,
    ticket_id BIGINT UNSIGNED NULL,
    last_payment_error VARCHAR(1000) NULL,
    expires_at DATETIME(3) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_reservations_expiry (status, expires_at),
    INDEX idx_reservations_user (user_id, created_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS outbox_events (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL UNIQUE,
    topic VARCHAR(255) NOT NULL,
    message_key VARCHAR(255) NOT NULL,
    payload JSON NOT NULL,
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    available_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_until DATETIME NULL,
    published_at DATETIME NULL,
    last_error VARCHAR(1000) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_outbox_pending (published_at, available_at, locked_until, id)
) ENGINE=InnoDB;

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
