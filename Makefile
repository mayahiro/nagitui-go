.PHONY: build check format format-check lint test

build:
	GOTOOLCHAIN=local go build ./...

test:
	GOTOOLCHAIN=local go test ./...

lint:
	GOTOOLCHAIN=local go vet ./...

format:
	GOWORK=off GOTOOLCHAIN=local go -C tools tool goimports -local github.com/mayahiro/nagitui-go -w ..

format-check:
	@unformatted="$$(find . -type f -name '*.go' -not -path './.git/*' -exec gofmt -l {} +)"; test -z "$$unformatted" || { printf '%s\n' "$$unformatted"; exit 1; }

check:
	$(MAKE) format-check
	$(MAKE) build
	$(MAKE) test
	$(MAKE) lint
