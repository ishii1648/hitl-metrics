.PHONY: build install uninstall grafana-fixtures grafana-up grafana-up-e2e grafana-down grafana-screenshot lint-dashboard worktree-create worktree-list worktree-remove

PREFIX ?= $(HOME)/.local
BIN_DIR := $(PREFIX)/bin
BIN_NAME := hitl-metrics

# 実データ表示用 DB パス（grafana-up が参照）。上書き可。
HITL_METRICS_DB ?= $(HOME)/.claude/hitl-metrics.db

build:
	CGO_ENABLED=0 go build -o bin/$(BIN_NAME) ./cmd/hitl-metrics/

install:
	@mkdir -p "$(BIN_DIR)"
	CGO_ENABLED=0 go build -o "$(BIN_DIR)/$(BIN_NAME)" ./cmd/hitl-metrics/
	@echo "Installed: $(BIN_DIR)/$(BIN_NAME)"
	@case ":$$PATH:" in *":$(BIN_DIR):"*) ;; *) echo "Warning: $(BIN_DIR) is not in PATH";; esac

uninstall:
	rm -f "$(BIN_DIR)/$(BIN_NAME)"
	@echo "Removed: $(BIN_DIR)/$(BIN_NAME)"

grafana-fixtures:
	CGO_ENABLED=0 GOTOOLCHAIN=local go test -run TestGenTestDB -v ./e2e/

grafana-up:
	@if [ ! -f "$(HITL_METRICS_DB)" ]; then \
		echo "DB not found: $(HITL_METRICS_DB)"; \
		echo "Run 'hitl-metrics sync-db' first, or override: make grafana-up HITL_METRICS_DB=/path/to/db"; \
		exit 1; \
	fi
	HITL_METRICS_DB=$(HITL_METRICS_DB) docker compose up -d
	@echo "Waiting for Grafana to be ready..."
	@for i in $$(seq 1 60); do \
		if curl -sf http://localhost:$${GRAFANA_PORT:-13000}/api/health > /dev/null 2>&1; then \
			echo "Grafana is ready at http://localhost:$${GRAFANA_PORT:-13000}"; \
			echo "Showing data from: $(HITL_METRICS_DB)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Grafana failed to start within 60s"; exit 1

grafana-up-e2e: grafana-fixtures
	HITL_METRICS_DB=$(CURDIR)/e2e/testdata/hitl-metrics.db docker compose up -d
	@echo "Waiting for Grafana to be ready..."
	@for i in $$(seq 1 60); do \
		if curl -sf http://localhost:$${GRAFANA_PORT:-13000}/api/health > /dev/null 2>&1; then \
			echo "Grafana is ready at http://localhost:$${GRAFANA_PORT:-13000} (e2e fixtures)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Grafana failed to start within 60s"; exit 1

grafana-down:
	docker compose down

grafana-screenshot: grafana-up-e2e
	bash e2e/screenshot.sh .outputs/grafana-screenshots

lint-dashboard:
	go run github.com/grafana/dashboard-linter@latest lint --strict --config grafana/dashboards/.lint grafana/dashboards/hitl-metrics.json

# Worktree management (usage: make worktree-create BRANCH=feat/atomic-write)
# Path convention: <repo_root>@<branch_dir_name> (gw_add と同じ @ 区切り)
MAIN_WORKTREE := $(shell git worktree list --porcelain | head -1 | sed 's/worktree //')
WT_DIR_NAME = $(subst /,-,$(BRANCH))
WT_PATH = $(MAIN_WORKTREE)@$(WT_DIR_NAME)

worktree-create:
	@if [ -z "$(BRANCH)" ]; then echo "Usage: make worktree-create BRANCH=feat/atomic-write"; exit 1; fi
	git fetch origin
	git worktree add "$(WT_PATH)" -b "$(BRANCH)" origin/HEAD
	@if [ -f .claude/settings.local.json ]; then \
		mkdir -p "$(WT_PATH)/.claude"; \
		cp .claude/settings.local.json "$(WT_PATH)/.claude/settings.local.json"; \
		echo "Copied .claude/settings.local.json"; \
	fi
	@echo "Worktree created: $(WT_PATH) (branch: $(BRANCH))"

worktree-list:
	git worktree list

worktree-remove:
	@if [ -z "$(BRANCH)" ]; then echo "Usage: make worktree-remove BRANCH=feat/atomic-write"; exit 1; fi
	git worktree remove "$(WT_PATH)"
