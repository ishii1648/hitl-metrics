.PHONY: grafana-fixtures grafana-up grafana-down grafana-screenshot lint-dashboard worktree-create worktree-list worktree-remove

grafana-fixtures:
	CGO_ENABLED=0 GOTOOLCHAIN=local go test -run TestGenTestDB -v ./e2e/

grafana-up: grafana-fixtures
	docker compose up -d
	@echo "Waiting for Grafana to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do \
		if curl -sf http://localhost:13000/api/health > /dev/null 2>&1; then \
			echo "Grafana is ready at http://localhost:13000"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Grafana failed to start within 30s"; exit 1

grafana-down:
	docker compose down

grafana-screenshot: grafana-up
	bash e2e/screenshot.sh .outputs/grafana-screenshots

lint-dashboard:
	go run github.com/grafana/dashboard-linter@latest lint --strict --config grafana/dashboards/.lint grafana/dashboards/claudedog.json

# Worktree management (usage: make worktree-create ADR=017)
# Path convention: <repo_root>@feat-adr-<NNN> (gw_add と同じ @ 区切り)
MAIN_WORKTREE := $(shell git worktree list --porcelain | head -1 | sed 's/worktree //')
WT_BRANCH = feat/adr-$(ADR)
WT_DIR_NAME = feat-adr-$(ADR)
WT_PATH = $(MAIN_WORKTREE)@$(WT_DIR_NAME)

worktree-create:
	@if [ -z "$(ADR)" ]; then echo "Usage: make worktree-create ADR=017"; exit 1; fi
	@adr_file=$$(ls docs/adr/$(ADR)-*.md 2>/dev/null | head -1); \
	if [ -z "$$adr_file" ]; then echo "Error: ADR-$(ADR) not found in docs/adr/"; exit 1; fi
	git fetch origin
	git worktree add "$(WT_PATH)" -b "$(WT_BRANCH)" origin/HEAD
	@if [ -f .claude/settings.local.json ]; then \
		mkdir -p "$(WT_PATH)/.claude"; \
		cp .claude/settings.local.json "$(WT_PATH)/.claude/settings.local.json"; \
		echo "Copied .claude/settings.local.json"; \
	fi
	@echo "Worktree created: $(WT_PATH) (branch: $(WT_BRANCH))"

worktree-list:
	git worktree list

worktree-remove:
	@if [ -z "$(ADR)" ]; then echo "Usage: make worktree-remove ADR=017"; exit 1; fi
	git worktree remove "$(WT_PATH)"
