# Rebuild & restart containers (fast path for code-only changes)
rebuild:
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-crawler axora-extractor

# New dependency → rebuild images, then restart containers
deps:
	DOCKER_BUILDKIT=1 docker compose build axora-crawler axora-extractor
	DOCKER_BUILDKIT=1 docker compose up -d axora-crawler axora-extractor

# Base image or system dependency change → force full rebuild
clean-build:
	DOCKER_BUILDKIT=1 docker compose build --no-cache axora-crawler axora-extractor
	DOCKER_BUILDKIT=1 docker compose up -d axora-crawler axora-extractor

# Run everything (without forcing rebuild unless image missing)
run:
	DOCKER_BUILDKIT=1 docker compose up -d

# Stop all containers
stop:
	docker compose down


MIGRATIONS_PATH = ./migrations

install-migrate:
	@echo "Installing golang-migrate..."
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: mig
# Create a new migration file with timestamp
# Usage: make mig <migration_name>
# Example: make mig crawl_url
mig:
	@if [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then \
		echo "Error: Please provide a migration name"; \
		echo "Usage: make mig <migration_name>"; \
		echo "Example: make mig crawl_url"; \
		exit 1; \
	fi
	@mkdir -p $(MIGRATIONS_PATH)
	@timestamp=$$(date -u +%Y%m%d%H%M%S); \
	migration_name=$(filter-out $@,$(MAKECMDGOALS)); \
	up_file="$(MIGRATIONS_PATH)/$${timestamp}_$${migration_name}.up.sql"; \
	down_file="$(MIGRATIONS_PATH)/$${timestamp}_$${migration_name}.down.sql"; \
	touch $$up_file $$down_file; \
	echo "Created migration files:"; \
	echo "  - $$up_file"; \
	echo "  - $$down_file"