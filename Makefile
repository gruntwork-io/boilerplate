GOFMT_FILES?=$$(find . -name '*.go' -not -path "./examples/*")

default: build

build: $(shell find . \( -type f -name '*.go' -print \))
	set -xe ;\
	vtag=$$(git describe --tags --abbrev=12 --dirty --broken) ;\
	go build -o boilerplate -ldflags "-X github.com/gruntwork-io/boilerplate/version.Version=$${vtag} -extldflags '-static'" .

clean:
	rm -f boilerplate

lint:
	golangci-lint run ./...

update-lint-config: SHELL:=/bin/bash
update-lint-config:
	curl -s https://raw.githubusercontent.com/gruntwork-io/terragrunt/main/.golangci.yml --output .golangci.yml
	tmpfile=$$(mktemp) ;\
	{ echo '# This file is generated from https://github.com/gruntwork-io/terragrunt/blob/main/.golangci.yml' ;\
	  echo '# It is automatically updated weekly via the update-lint-config workflow. Do not edit manually.' ;\
	  cat .golangci.yml; } > $${tmpfile} && mv $${tmpfile} .golangci.yml

test:
	go test -v ./...

fmt:
	@echo "Running source files through gofmt..."
	gofmt -w $(GOFMT_FILES)

build-wasm:
	GOOS=js GOARCH=wasm go build -o examples/wasm/boilerplate.wasm -ldflags "-s -w" ./cmd/wasm/

compress-wasm: build-wasm
	@command -v brotli >/dev/null 2>&1 || { echo "Error: brotli CLI not found. Install with: brew install brotli (macOS) or apt-get install brotli (Linux)"; exit 1; }
	brotli --best --force examples/wasm/boilerplate.wasm -o examples/wasm/boilerplate.wasm.br
	@echo "Uncompressed: $$(wc -c < examples/wasm/boilerplate.wasm | tr -d ' ') bytes"
	@echo "Compressed:   $$(wc -c < examples/wasm/boilerplate.wasm.br | tr -d ' ') bytes"

copy-wasm-exec:
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" examples/wasm/

wasm: compress-wasm copy-wasm-exec
	@echo "WASM build complete:"
	@ls -lh examples/wasm/boilerplate.wasm examples/wasm/boilerplate.wasm.br

.PHONY: lint test default update-lint-config build-wasm compress-wasm copy-wasm-exec wasm
