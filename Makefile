.PHONY: help deps migrate run-api run-graph run-ingest run-web docker-up docker-down build test \
	phase0-setup phase0-verify phase1-verify dev-all

help:
	@echo "Targets:"
	@echo "  docker-up, migrate, run-api, run-graph, run-ingest, run-web"
	@echo "  phase0-setup  - kind cluster + demo app"
	@echo "  phase0-verify - check Phase 0 exit criteria"
	@echo "  phase1-verify - smoke test change ingest"

deps:
	go mod tidy
	cd web && npm install

migrate:
	@cat migrations/001_init.sql | docker compose -f deploy/docker-compose.yml exec -T postgres psql -U kube -d kubedashboard 2>/dev/null || \
	(test -n "$$DATABASE_URL" || export DATABASE_URL=postgres://kube:kube@localhost:5433/kubedashboard?sslmode=disable; \
	psql "$$DATABASE_URL" -f migrations/001_init.sql)

docker-up:
	docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

run-api:
	go run ./cmd/api

run-graph:
	go run ./cmd/graph

run-ingest:
	go run ./cmd/ingest

run-web:
	cd web && npm run dev

build:
	go build -o bin/api ./cmd/api
	go build -o bin/graph ./cmd/graph
	go build -o bin/ingest ./cmd/ingest
	cd web && npm run build

test:
	go test ./...

phase0-setup:
	chmod +x scripts/*.sh && ./scripts/phase0-setup-kind.sh

phase0-verify:
	chmod +x scripts/*.sh && ./scripts/phase0-verify.sh

phase1-verify:
	chmod +x scripts/*.sh && ./scripts/phase1-verify.sh
