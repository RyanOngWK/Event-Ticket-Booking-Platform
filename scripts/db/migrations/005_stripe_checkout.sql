-- Expand the reservation-unit model into a timed checkout lifecycle.
ALTER TABLE event_db.events
    ADD COLUMN price_cents BIGINT UNSIGNED NOT NULL DEFAULT 2500,
    ADD COLUMN currency CHAR(3) NOT NULL DEFAULT 'usd';

ALTER TABLE event_db.reservation_units
    MODIFY COLUMN status ENUM('available', 'reserved', 'sold') NOT NULL DEFAULT 'available',
    ADD COLUMN reservation_id CHAR(36) NULL AFTER status;

CREATE TABLE IF NOT EXISTS ticket_db.reservations (
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
