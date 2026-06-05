# KubeAtlas — common dev commands (run from repo root)

DATABASE_URL ?= postgres://kube:kube@localhost:5432/kubedashboard?sslmode=disable
API_ADDR     ?= :8080
CLUSTER_ID   ?= local
AI_SERVICE_URL ?= http://localhost:8090
WEB_PORT     ?= 5180

export DATABASE_URL API_ADDR CLUSTER_ID AI_SERVICE_URL

.PHONY: help up down migrate build test check \
	run-api run-worker run-ai run-web setup-ai

help:
	@echo "KubeAtlas — run from repo root"
	@echo ""
	@echo "  make up          Postgres (+ optional AI container)"
	@echo "  make migrate     Apply all SQL migrations"
	@echo "  make setup-ai    Python venv for AI service"
	@echo "  make run-api     Go REST API + SSE  (:8080)"
	@echo "  make run-ai      Python FastAPI     (:8090)"
	@echo "  make run-worker  Go cluster pipeline (needs kubectl)"
	@echo "  make run-web     React UI           (:5180)"
	@echo "  make check       Health-check API + AI + DB"
	@echo "  make build       Compile Go + web"
	@echo ""
	@echo "See docs/RUN.md for the full 4-terminal flow."

up:
	docker compose -f deploy/docker-compose.yml up -d postgres

down:
	docker compose -f deploy/docker-compose.yml down

migrate:
	@for f in migrations/001_init.sql migrations/002_add_graph_node_id.sql migrations/003_kubeatlas.sql; do \
		echo "==> $$f"; \
		docker compose -f deploy/docker-compose.yml exec -T postgres \
			psql -U kube -d kubedashboard -v ON_ERROR_STOP=1 < $$f || exit 1; \
	done
	@echo "Migrations OK"

setup-ai:
	@test -d services/ai/.venv || python3 -m venv services/ai/.venv
	services/ai/.venv/bin/pip install -q -r services/ai/requirements.txt
	@echo "AI venv ready: services/ai/.venv"

run-api:
	go run ./cmd/api

export GROQ_API_KEY=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

run-ai: setup-ai
	services/ai/.venv/bin/uvicorn app.main:app \
	--app-dir services/ai \
	--host 0.0.0.0 \
	--port 8090

run-worker:
	go run ./cmd/worker

run-web:
	cd web && npm run dev -- --host 0.0.0.0 --port $(WEB_PORT)

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	cd web && npm run build

test:
	go test ./...
	@$(MAKE) setup-ai
	cd services/ai && .venv/bin/python -m pytest tests -q

check:
	@bash scripts/check-pipeline.sh
