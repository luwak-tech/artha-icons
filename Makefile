.PHONY: build run test clean docker run-docker

# Binary name
APP_NAME=artha-icons

# Build the executable
build:
	@echo "Building $(APP_NAME)..."
	@go build -o bin/$(APP_NAME) ./cmd/sync/...

# Run the project locally
run: build
	@echo "Running $(APP_NAME)..."
	@./bin/$(APP_NAME)

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean the binary
clean:
	@echo "Cleaning up..."
	@rm -rf bin/

# Build docker image
docker:
	@echo "Building docker image..."
	@docker build -t $(APP_NAME):latest .

# Run docker container (one-off sync)
run-docker: docker
	@echo "Running docker container..."
	@docker run --rm -v $(PWD)/logos:/app/logos -v $(PWD)/data:/app/data $(APP_NAME):latest
