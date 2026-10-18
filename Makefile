.PHONY: up down build test-unit test-integration test-concurrency test-e2e seed migrate-fencing migrate-outbox logs-% clean

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

test-concurrency:
	go test -tags=integration ./services/ticket/tests/integration/... -run TestConcurrency -v -count=1

test-e2e:
	go test -tags=e2e ./services/e2e/... -v -count=1

seed:
	go run scripts/seed/seed_events.go

migrate-fencing:
	docker compose exec -T mysql mysql -u root -prootpassword < scripts/db/migrations/002_event_fencing_tokens.sql

migrate-outbox:
	docker compose exec -T mysql mysql -u root -prootpassword < scripts/db/migrations/003_ticket_outbox.sql

logs-%:
	docker compose logs $*

clean:
	docker compose down -v --rmi local --remove-orphans
