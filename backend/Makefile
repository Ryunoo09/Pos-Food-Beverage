.PHONY: build run seed test lint fmt tidy migrate-up migrate-down migrate-create backup backup-encrypted

BIN := bin/server
MIGRATION_DIR := internal/db/migrations

build:
	go build -o $(BIN) ./cmd/server

run:
	go run ./cmd/server

# Data dev: bisnis, outlet, 4 role default, dan akun dummy untuk login.
seed:
	go run ./cmd/seed

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

tidy:
	go mod tidy

# Butuh migrate CLI (golang-migrate) dan DATABASE_URL di environment.
migrate-up:
	migrate -path $(MIGRATION_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path $(MIGRATION_DIR) -database "$(DATABASE_URL)" down 1

migrate-create:
	migrate create -ext sql -dir $(MIGRATION_DIR) -seq $(name)

# Butuh pg_dump. Sesuai plan 09.
backup:
	pg_dump -Fc -Z9 "$(DATABASE_URL)" -f "backups/backup_$$(date +%Y%m%d_%H%M%S).dump"

backup-encrypted:
	pg_dump -Fc "$(DATABASE_URL)" | openssl enc -aes-256-cbc -pbkdf2 -k "$(BACKUP_ENCRYPTION_KEY)" \
		-out "backups/backup_$$(date +%Y%m%d_%H%M%S).dump.enc"
