.PHONY: help
help: ## Displays help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-z0-9A-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: deploy
deploy: ## init
	@bash setup-gke.sh bwplotka-stdtest

.PHONY: run
run: ## run brokenapp
	@go run brokenapp.go -listen-address=":99"

THE_CHOSEN_POD=$(shell kubectl get pods -l 'app=brokenapp' -o jsonpath='{.items[*].metadata.name}' | sort | awk '{print $$3}')

.PHONY: port-forward
port-forward: ## forward traffic to one pod
	@kubectl port-forward pod/$(THE_CHOSEN_POD) 9999

.PHONY: metrics
metrics: ## get metrics from one pod
	@kubectl exec $(THE_CHOSEN_POD) -- curl -s http://localhost:9999/metrics

.PHONY: open-metrics
open-metrics: ## get metrics from one pod
	@kubectl exec $(THE_CHOSEN_POD) -- curl -s -H 'Accept: application/openmetrics-text' http://localhost:9999/metrics

CASE=0
.PHONY: break
break: ## break one pod
	@kubectl exec $(THE_CHOSEN_POD) -- curl -s http://localhost:9999/break/$(CASE)

.PHONY: fix
fix: ## fix one pod
	@kubectl exec $(THE_CHOSEN_POD) -- curl -s http://localhost:9999/fix/$(CASE)
