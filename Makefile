.PHONY: bench build check format format-check lint test

bench:
	GOTOOLCHAIN=local go test -run '^$$' -bench '^Benchmark((ScrollViewport(Eager|Virtual|VirtualStickToEndGrowth|VirtualIdentified)|VirtualFlowVariableHeight)100K|ContentProjection(VisibleSubtree|BoundedFailure100K))$$' -benchmem -benchtime=5x -count=3 .
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkClipboardEncoding(Small|1MiB)$$' -benchmem -benchtime=1s -count=3 .
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkPointerTextHit100K$$' -benchmem -benchtime=1s -count=3 .
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkSuggestionPopupCandidates$$' -benchmem -benchtime=10000x -count=3 ./widget
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkJSONDocument100K$$' -benchmem -benchtime=5x -count=3 ./widget
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkJSONInspectorViewport$$' -benchmem -benchtime=10000x -count=3 ./widget

build:
	GOTOOLCHAIN=local go build ./...

test:
	GOTOOLCHAIN=local go test ./...

lint:
	GOTOOLCHAIN=local go vet ./...

format:
	find . -type f -name '*.go' -not -path './.git/*' -exec gofmt -w {} +

format-check:
	@unformatted="$$(find . -type f -name '*.go' -not -path './.git/*' -exec gofmt -l {} +)"; test -z "$$unformatted" || { printf '%s\n' "$$unformatted"; exit 1; }

check:
	$(MAKE) format-check
	$(MAKE) build
	$(MAKE) test
	$(MAKE) lint
