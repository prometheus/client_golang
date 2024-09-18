.PHONY: help
help: ## Displays help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-z0-9A-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: init
init: ## init
	$(MAKE) stop
	go test -count 1 -v -tags=interactive -run ^TestPlayground ./internal & # count 1 is to avoid cache.

.PHONY: stop
stop: ## stop init
	@curl -s http://localhost:19920 || true

.PHONY: run
run: ## run whatsup
	@go run main.go -config-file=./whatsup.yaml -address=:99

.PHONY: metrics
metrics: ## get metrics from whatsup
	@curl -H 'Accept: application/openmetrics-text' localhost:99/metrics

.PHONY: whatsup
whatsup: ## ask what's up
	@curl localhost:99/whatsup

.PHONY: test
test: ## run acceptance tests
	go test -count 1 -v -tags=interactive -run ^TestAcceptance ./internal & # count 1 is to avoid cache.