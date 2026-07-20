.PHONY: build check format format-check lint test

build:
	@packages="$$(GOTOOLCHAIN=local go list ./...)"; if [ -n "$$packages" ]; then GOTOOLCHAIN=local go build $$packages; else printf '%s\n' "No Go packages in the Phase F0 scaffold"; fi

test:
	@packages="$$(GOTOOLCHAIN=local go list ./...)"; if [ -n "$$packages" ]; then GOTOOLCHAIN=local go test $$packages; else printf '%s\n' "No Go packages in the Phase F0 scaffold"; fi

lint:
	@packages="$$(GOTOOLCHAIN=local go list ./...)"; if [ -n "$$packages" ]; then GOTOOLCHAIN=local go vet $$packages; else printf '%s\n' "No Go packages in the Phase F0 scaffold"; fi

format:
	find . -type f -name '*.go' -not -path './.git/*' -exec gofmt -w {} +

format-check:
	@unformatted="$$(find . -type f -name '*.go' -not -path './.git/*' -exec gofmt -l {} +)"; test -z "$$unformatted" || { printf '%s\n' "$$unformatted"; exit 1; }

check:
	$(MAKE) format-check
	$(MAKE) build
	$(MAKE) test
	$(MAKE) lint
