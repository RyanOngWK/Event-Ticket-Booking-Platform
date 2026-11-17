.PHONY: up down build test-unit test-integration test-e2e simulate-reservations seed migrate-reservation-units migrate-outbox migrate-stripe-checkout logs-% clean

up:
	docker compose up -d --build

down:
	docker compose down -v

build:
	docker compose build

test-unit:
	go test ./services/... -v

test-integration:
	go test ./services/... -tags=integration -v

test-e2e:
	go test -tags=e2e ./services/e2e/... -v -count=1

simulate-reservations:
	go run ./services/ticket/cmd/loadsim -users 10000 -inventory 500

seed:
	go run scripts/seed/seed_events.go

migrate-reservation-units:
	docker compose exec -T mysql mysql -u root -prootpassword < scripts/db/migrations/004_reservation_units.sql

migrate-outbox:
	docker compose exec -T mysql mysql -u root -prootpassword < scripts/db/migrations/003_ticket_outbox.sql

# One-time migration for existing databases already initialized before checkout support.
migrate-stripe-checkout:
	docker compose exec -T mysql mysql -u root -prootpassword < scripts/db/migrations/005_stripe_checkout.sql

logs-%:
	docker compose logs $*

clean:
	docker compose down -v --rmi local --remove-orphans
