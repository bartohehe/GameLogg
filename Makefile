# Wymaga: Go, Node (npm), Docker z compose v2, GNU make i powłoki sh.

DEV_DATABASE_URL  ?= postgres://gamelogg:gamelogg-dev@localhost:5432/gamelogg?sslmode=disable
TEST_DATABASE_URL ?= postgres://gamelogg:gamelogg-test@localhost:5433/gamelogg_test?sslmode=disable
DEV_JWT_SECRET    ?= dev-only-secret-do-not-use-in-production
VERSION           ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS           := -s -w -X main.version=$(VERSION)
DEV_COMPOSE       := docker compose -f docker-compose.dev.yml

.PHONY: dev dev-db test test-short test-db lint build docker clean

web/node_modules: web/package.json web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

## dev: baza w Dockerze, API na :8080 (go run) i frontend z HMR na :5173
dev: dev-db web/node_modules
	@echo "Otwórz http://localhost:5173 (Vite przekazuje /api do :8080). Ctrl+C kończy oba procesy."
	@trap 'kill 0' INT TERM EXIT; \
	(cd web && npm run dev) & \
	APP_ENV=dev DATABASE_URL="$(DEV_DATABASE_URL)" \
	JWT_SECRET="$${JWT_SECRET:-$(DEV_JWT_SECRET)}" \
	go run -ldflags "-X main.version=$(VERSION)" ./cmd/gamelogg

dev-db:
	$(DEV_COMPOSE) up -d --wait postgres

## test: vet, wszystkie testy Go (z integracyjnymi na Postgresie) i testy frontendu
test: test-db web/node_modules
	go vet ./...
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...
	cd web && npm test

## test-short: bez Dockera, pomija testy integracyjne
test-short: web/node_modules
	go vet ./...
	go test -short ./...
	cd web && npm test

test-db:
	$(DEV_COMPOSE) up -d --wait postgres-test

lint: web/node_modules
	go vet ./...
	cd web && npm run lint

## build: frontend + binarka z wbudowanym frontendem w bin/gamelogg
build: web/node_modules
	cd web && npm run build
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/gamelogg ./cmd/gamelogg

## docker: obraz produkcyjny gamelogg:$(VERSION)
docker:
	docker build --build-arg VERSION=$(VERSION) -t gamelogg:$(VERSION) .

clean:
	rm -rf bin web/node_modules/.vite
	find web/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
