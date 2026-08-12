.PHONY: bench build check format format-check lint test

bench:
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkInheritedOptions(SelectedPath|100UnrelatedBranches)$$' -benchmem -benchtime=100x -count=3 .

build:
	GOTOOLCHAIN=local go build ./...

test:
	GOTOOLCHAIN=local go test -count=1 ./...

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
