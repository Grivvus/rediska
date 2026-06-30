.PHONY: test
test:
	@echo "run all tests"
	@go test --shuffle=on -race -coverprofile=coverage.txt -v ./...

.PHONY: lint
lint:
	@echo "starting golangci-lint in docker"
	@docker run -t --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v2.11.3 golangci-lint run
