BIN ?= bin/health-exporter

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

tidy:
	go mod tidy

test: fmt vet tidy ## Run linters and tests.
	golangci-lint run
	go test ./... -covermode=atomic -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

##@ Build

build: test ## Build binary.
	go build -o $(BIN) ./cmd/health-exporter

run: fmt vet ## Run the exporter locally.
	go run ./cmd/health-exporter

docker-build: test ## Build docker image with the exporter.
	sudo podman build -t ${IMG} .

docker-push: ## Push docker image with the exporter.
	sudo podman push ${IMG}

docker-login:
	sudo podman login ${REG} -u ${REG_USER} -p ${REG_PASSWORD}

redeploy: docker-build docker-login docker-push
