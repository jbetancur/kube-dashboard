# Variables$
APP_NAME_API = kube-dashboard-api
APP_NAME_TUI = kube-dashboard-tui
BUILD_DIR = build
GO_CMD = go

# Default target$
.PHONY: all
all: build-api build-tui

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
$
# Run the TUI
.PHONY: run-tui
run-tui:
	@echo "Running TUI..."
	$(GO_CMD) run ./cmd/tui%