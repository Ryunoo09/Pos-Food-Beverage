.PHONY: build run seed test lint fmt tidy migrate-up migrate-down migrate-create backup backup-encrypted verify-backup restore-test

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
	@TIMESTAMP=$$(date +%Y%m%d_%H%M%S); \
	FILE=backups/backup_$$TIMESTAMP.dump; \
	pg_dump -Fc -Z9 "$(DATABASE_URL)" -f "$$FILE"; \
	sha256sum "$$FILE" > "$$FILE.sha256"; \
	echo "Backup: $$FILE"

backup-encrypted:
	@TIMESTAMP=$$(date +%Y%m%d_%H%M%S); \
	FILE=backups/backup_$$TIMESTAMP.dump.enc; \
	pg_dump -Fc "$(DATABASE_URL)" | openssl enc -aes-256-cbc -pbkdf2 -pass "env:BACKUP_ENCRYPTION_KEY" -out "$$FILE"; \
	sha256sum "$$FILE" > "$$FILE.sha256"; \
	echo "Backup: $$FILE"

# Pakai: make verify-backup FILE=backups/backup_....dump.enc
verify-backup:
	cd backups && sha256sum -c "$(notdir $(FILE)).sha256"

# Uji file backup bisa dibaca pg_restore tanpa menyentuh DB.
restore-test:
	@if [ "${FILE##*.}" = "enc" ]; then \
		openssl enc -d -aes-256-cbc -pbkdf2 -pass "env:BACKUP_ENCRYPTION_KEY" -in "$(FILE)" | pg_restore --list > /dev/null; \
	else \
		pg_restore --list "$(FILE)" > /dev/null; \
	fi && echo "Restore test OK"
