all:
	DOCKER_BUILDKIT=1 docker compose up -d --build
	
run:
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-crawler

ins:
	go mod tidy && go mod vendor

go:
	docker compose up -d

nocache:
	DOCKER_BUILDKIT=1 docker compose build --no-cache axora-crawler

debug:
	docker compose -f docker-compose.yaml up --build

sh:
	docker exec -it axora-crawler /bin/sh