.PHONY: help build test run docker-up docker-down

BINARY_NAME=b3-analyzer
GO_FILES=$(shell find . -name '*.go' -type f)

help: ## Mostra este help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $1, $2}'

build: ## Compila o binário
	@echo "Building..."
	go build -ldflags="-s -w" -o bin/$(BINARY_NAME) cmd/api/main.go
	go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-cli cmd/cli/main.go

build-linux: ## Compila para Linux
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-linux cmd/api/main.go

test: ## Roda os testes
	@echo "Testing..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

bench: ## Roda benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

lint: ## Roda o linter
	@echo "Linting..."
	golangci-lint run --enable-all

run: ## Roda a API
	@echo "Starting API..."
	go run cmd/api/main.go

docker-up: ## Sobe os containers
	docker-compose up -d

docker-down: ## Para os containers
	docker-compose down

migrate-up: ## Roda migrations
	migrate -path migrations -database "postgres://localhost/b3_market?sslmode=disable" up

migrate-down: ## Reverte migrations
	migrate -path migrations -database "postgres://localhost/b3_market?sslmode=disable" down

clean: ## Limpa arquivos temporários
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

install-tools: ## Instala ferramentas de desenvolvimento
	@echo "Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest