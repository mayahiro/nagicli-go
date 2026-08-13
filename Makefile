.PHONY: bench build check format format-check lint test

bench:
	GOTOOLCHAIN=local go test -run '^$$' -bench '^(BenchmarkInheritedOptions(SelectedPath|100UnrelatedBranches)|BenchmarkDeprecatedOption1000Occurrences)$$' -benchmem -benchtime=1000x -count=3 .
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkCompletion(SelectedPath|100UnrelatedBranches|NoVisibleDeclarations|1000HiddenPairs)$$' -benchmem -benchtime=10000x -count=3 .
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkJSONDiagnosticRenderer$$' -benchmem -benchtime=100x -count=3 .
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkPrompt(Confirm|Input64KiB)$$' -benchmem -benchtime=100x -count=3 ./prompt
	GOTOOLCHAIN=local go test -run '^$$' -bench '^BenchmarkStatus(TerminalUpdate|LogCoalesced)$$' -benchmem -benchtime=10000x -count=3 ./status

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
