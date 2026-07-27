APP_NAME=gptcode
APP_PATH=./cmd/gptcode

GOBIN?=$(shell go env GOBIN)
ifeq ($(GOBIN),)
    GOBIN=$(HOME)/go/bin
endif

ML_DIR=ml/complexity_detection

.PHONY: all build install dev clean test evidence verify public-contract train-ml train-complexity install-ml

all: build

build:
	@echo "-> Building $(APP_NAME)..."
	@go build -o bin/$(APP_NAME) $(APP_PATH)

install: build
	@echo "-> Installing $(APP_NAME) to $(GOBIN)..."
	@mkdir -p $(GOBIN)
	@cp bin/$(APP_NAME) $(GOBIN)/
	@echo "-> Running gptcode setup..."
	@$(GOBIN)/$(APP_NAME) setup
	@echo "-> Done."

install-ml: install
	@echo "-> Setting up ML models..."
	@$(MAKE) train-complexity
	@echo "-> ML models ready."

dev:
	@echo "-> Running in dev mode..."
	@go run $(APP_PATH)

clean:
	@echo "-> Cleaning..."
	@rm -rf bin/
	@rm -rf $(ML_DIR)/venv
	@rm -f $(ML_DIR)/models/*.json

test:
	@echo "-> Running Go tests..."
	@go test -short ./...

public-contract:
	@./scripts/test-public-contract.sh

evidence:
	@./scripts/verify-public-examples.sh

verify:
	@echo "-> Checking Go formatting..."
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"
	@echo "-> Running go vet..."
	@go vet ./...
	@echo "-> Building CLI..."
	@mkdir -p bin
	@go build -o bin/gptcode $(APP_PATH)
	@echo "-> Running test suite..."
	@PATH="$(CURDIR)/bin:$$PATH" go test -short ./...
	@$(MAKE) public-contract
	@$(MAKE) evidence

# ML Training targets
train-ml:
	@gptcode train

train-complexity:
	@gptcode train complexity_detection
