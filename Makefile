rd:
	DOCKER_BUILDKIT=1 docker compose up -d axora-qdrant 
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-crawler 
	
run:
	DOCKER_BUILDKIT=1 docker compose up -d

all:
	DOCKER_BUILDKIT=1 docker compose up -d --build 

ins:
	go mod tidy && go mod vendor

pprof:
	go tool pprof -http=:8082 http://localhost:8082/debug/pprof/heap

goroutine:
	go tool pprof -http=:8083 http://localhost:8082/debug/pprof/goroutine

cpu:
	go tool pprof -http=:8084 http://localhost:8082/debug/pprof/profile

test:
	go tool pprof http://localhost:8082/debug/pprof/goroutine
	go tool pprof http://localhost:8082/debug/pprof/heap