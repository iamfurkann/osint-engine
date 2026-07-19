.PHONY: build clean test lint release

APP_NAME_CLI = osint
APP_NAME_DAEMON = osintd
BIN_DIR = bin

build:
	@echo "Building $(APP_NAME_CLI)..."
	go build -o $(BIN_DIR)/$(APP_NAME_CLI) ./cmd/osint
	@echo "Building $(APP_NAME_DAEMON)..."
	go build -o $(BIN_DIR)/$(APP_NAME_DAEMON) ./cmd/osintd

clean:
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)

test:
	@echo "Running tests..."
	go test -v ./...

lint:
	@echo "Running linter..."
	golangci-lint run ./...

release: clean
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME_CLI)-linux-amd64 ./cmd/osint
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME_DAEMON)-linux-amd64 ./cmd/osintd
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME_CLI)-windows-amd64.exe ./cmd/osint
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME_CLI)-darwin-amd64 ./cmd/osint
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/$(APP_NAME_CLI)-darwin-arm64 ./cmd/osint