# Variables$
APP_NAME_API = kube-dashboard-api
APP_NAME_TUI = kube-dashboard-tui
BUILD_DIR = build
PLUGIN_BUILD_DIR = ${BUILD_DIR}/plugins
PLUGINS_PROVIDERS = ./plugins/providers
GO_CMD = go

# Default target
.PHONY: all
all: build-api build-tui build-plugins

# Build the REST API
.PHONY: build-api
build-api:
	@echo "Building REST API..."
	$(GO_CMD) build -o $(BUILD_DIR)/$(APP_NAME_API) ./cmd/rest-api$
	@echo "REST API built successfully: $(BUILD_DIR)/$(APP_NAME_API)"$

# Build the TUI
.PHONY: build-tui
build-tui:
	@echo "Building TUI..."
	$(GO_CMD) build -o $(BUILD_DIR)/$(APP_NAME_TUI) ./cmd/tui$
	@echo "TUI built successfully: $(BUILD_DIR)/$(APP_NAME_TUI)"$

# Build plugins
.PHONY: build-plugins
build-plugins:
	@echo "Building plugins..."
	mkdir -p $(PLUGIN_BUILD_DIR)

	@for file in $(PLUGINS_PROVIDERS)/*.go; do \
		plugin_name=$$(basename $$file .go); \
		echo "Building plugin: $$plugin_name"; \
		$(GO_CMD) build -buildmode=plugin -o $(PLUGIN_BUILD_DIR)/$$plugin_name.so $$file; \
	done

	@echo "Plugins built successfully: $(PLUGIN_BUILD_DIR)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	@echo "Cleaned successfully."

# Run the REST API
.PHONY: run-api
run-api:
	@echo "Running REST API..."
	$(GO_CMD) run ./cmd/rest-api

# Run the Agent
.PHONY: run-agent
run-agent:
	@echo "Running Agent..."
	$(GO_CMD) run ./cmd/agent

# Run the TUI
.PHONY: run-tui
run-tui:
	@echo "Running TUI..."
	$(GO_CMD) run ./cmd/tui

# Generate gRPC code
.PHONY: generate-grpc
generate-grpc:
	@echo "Generating gRPC code..."
	protoc --go_out=. --go-grpc_out=. internal/pkg/grpc/message.proto
	@echo "gRPC code generated successfully."