rd:
	DOCKER_BUILDKIT=1 docker compose up -d axora-qdrant 
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-crawler 

run:
	DOCKER_BUILDKIT=1 docker compose up -d

all:
	DOCKER_BUILDKIT=1 docker compose up -d --build

ins:
	go mod tidy && go mod vendor

test:
	go tool pprof http://localhost:8002/debug/pprof/goroutine
	go tool pprof http://localhost:8002/debug/pprof/heap
	go tool pprof http://localhost:8002/debug/pprof/profile?seconds=30
	http://localhost:8002/debug/pprof/