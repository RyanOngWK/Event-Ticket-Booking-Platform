.PHONY: up down build test-unit test-integration test-concurrency test-e2e seed logs-% clean

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

logs-%:
	docker compose logs $*

clean:
	docker compose down -v --rmi local --remove-orphans
