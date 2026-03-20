.PHONY: grafana-fixtures grafana-up grafana-down grafana-screenshot

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
