NAME = health_exporter

.PHONY: vendor build run bash debug test fmt lint

SHELL = bash
IMAGE = $(NAME):local
COMPOSE = IMAGE=$(IMAGE) docker-compose
# Run once when you clone the repo or using a new module
vendor:
	go mod download && go mod tidy && go mod verify

build:
	docker build -t $(IMAGE) -f Dockerfile ./

run: build
	$(COMPOSE) up --force-recreate

clean: build
	$(COMPOSE) down

rsh:
	$(COMPOSE) exec --user root $(NAME) sh -c 'bash || sh'
	$(RUN) bash

debug:
	$(COMPOSE) run --entrypoint='sh' $(NAME) -c 'tail -f /dev/null'

# Run make build once before running tests
test:
	./test/test.sh

# Runs go-fmt on your codes except vendor and resources dirs
fmt:
	go fmt ./cmd/... ./pkg/...

# Lint checks your codes except vendor and resources dirs
lint:
	golint ./cmd/... ./pkg/...
