run:
	DOCKER_BUILDKIT=1 docker compose up -d --build

ins:
	go mod tidy && go mod vendor

go:
	docker compose up -d

nocache:
	DOCKER_BUILDKIT=1 docker compose build --no-cache axora-crawler
