rd:
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-crawler 
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-rag 

run:
	DOCKER_BUILDKIT=1 docker compose up -d

all:
	DOCKER_BUILDKIT=1 docker compose up -d --build 

ins:
	go mod tidy && go mod vendor

model:
	docker exec -it axora-ollama ollama pull mistral:7b-instruct-q4_0

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