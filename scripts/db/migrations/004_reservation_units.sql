-- Replace the per-event fencing counter and hot inventory row with one row
-- per inventory unit. Existing available inventory remains claimable, while
-- previously sold units are initialized as sold.
CREATE TABLE IF NOT EXISTS event_db.reservation_units (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_id BIGINT UNSIGNED NOT NULL,
    unit_number INT UNSIGNED NOT NULL,
    status ENUM('available', 'sold') NOT NULL DEFAULT 'available',
    ticket_id BIGINT UNSIGNED NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_event_unit (event_id, unit_number),
    INDEX idx_available_units (event_id, status, id)
) ENGINE=InnoDB;

SET SESSION cte_max_recursion_depth = 1000000;
INSERT IGNORE INTO event_db.reservation_units (event_id, unit_number, status)
WITH RECURSIVE unit_numbers (unit_number) AS (
    SELECT 1
    UNION ALL
    SELECT unit_number + 1 FROM unit_numbers
    WHERE unit_number < (SELECT COALESCE(MAX(total_capacity), 0) FROM event_db.events)
)
SELECT e.id, u.unit_number,
       IF(u.unit_number <= e.remaining_count, 'available', 'sold')
FROM event_db.events e
JOIN unit_numbers u ON u.unit_number <= e.total_capacity;

DROP TABLE IF EXISTS event_db.event_fencing_tokens;
