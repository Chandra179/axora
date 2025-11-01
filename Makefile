kill-ports:
	# Kill ports 8082–8084
	@lsof -ti :8082 | xargs -r kill -9 || true
	@lsof -ti :8083 | xargs -r kill -9 || true
	@lsof -ti :8084 | xargs -r kill -9 || true

.PHONY: rd wait-for-pprof

wait-for-pprof:
	@echo "⏳ waiting for pprof to be ready at localhost:8002..."
	@until curl -s http://localhost:8002/debug/pprof/ > /dev/null; do \
		printf "."; \
		sleep 1; \
	done
	@echo " ✅ pprof endpoint ready!"

rd: kill-ports
	DOCKER_BUILDKIT=1 docker compose up -d axora-qdrant 
	DOCKER_BUILDKIT=1 docker compose up -d --build axora-crawler 
	
	$(MAKE) wait-for-pprof

	go tool pprof -http=:8082 http://localhost:8002/debug/pprof/heap &
	go tool pprof -http=:8083 http://localhost:8002/debug/pprof/goroutine &
run:
	DOCKER_BUILDKIT=1 docker compose up -d

all:
	DOCKER_BUILDKIT=1 docker compose up -d --build

ins:
	go mod tidy && go mod vendor

heap:
	go tool pprof -http=:8082 http://localhost:8002/debug/pprof/heap &

gor:
	go tool pprof -http=:8083 http://localhost:8002/debug/pprof/goroutine &

cpu:
	go tool pprof -http=:8084 http://localhost:8002/debug/pprof/profile &

test:
	go tool pprof http://localhost:8002/debug/pprof/goroutine
	go tool pprof http://localhost:8002/debug/pprof/heap
	go tool pprof http://localhost:8002/debug/pprof/profile?seconds=30