run:
	DOCKER_BUILDKIT=1 docker compose up -d --build

ins:
	go mod tidy && go mod vendor