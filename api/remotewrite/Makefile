
.PHONY: proto
proto: ## Regenerate Go from proto.
proto: $(BUF)
	@echo ">> regenerating Prometheus proto"
	@$(BUF) generate
	# TODO(bwplotka): Is there a way to configure buf for this?
	@find genproto/ -type f -exec sed -i '' 's/package prompb/package writev1/g' {} \;
	# For some reasons buf generates this unused import, kill it manually for now and reformat.
	@find genproto/ -type f -exec sed -i '' 's/_ "github.com\/gogo\/protobuf\/gogoproto"//g' {} \;
	@go fmt ./genproto/...
