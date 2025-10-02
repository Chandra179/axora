all:
	DOCKER_BUILDKIT=1 docker compose up -d --build
	
run:
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-crawler

ins:
	go mod tidy && go mod vendor