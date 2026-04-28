GOFMT_FILES?=$$(find . -name '*.go' -not -path "./examples/*")
WASM_DIR?=examples/for-learning-and-testing/wasm

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

build-wasm-lite:
	GOOS=js GOARCH=wasm go build -o $(WASM_DIR)/boilerplate.wasm -ldflags "-s -w" ./cmd/wasm/lite/

build-wasm-full:
	GOOS=js GOARCH=wasm go build -o $(WASM_DIR)/boilerplate-full.wasm -ldflags "-s -w" ./cmd/wasm/full/

build-wasm: build-wasm-lite build-wasm-full

compress-wasm: build-wasm
	@command -v brotli >/dev/null 2>&1 || { echo "Error: brotli CLI not found. Install with: brew install brotli (macOS) or apt-get install brotli (Linux)"; exit 1; }
	brotli --best --force $(WASM_DIR)/boilerplate.wasm -o $(WASM_DIR)/boilerplate.wasm.br
	brotli --best --force $(WASM_DIR)/boilerplate-full.wasm -o $(WASM_DIR)/boilerplate-full.wasm.br
	@echo "Lite uncompressed: $$(wc -c < $(WASM_DIR)/boilerplate.wasm | tr -d ' ') bytes"
	@echo "Lite compressed:   $$(wc -c < $(WASM_DIR)/boilerplate.wasm.br | tr -d ' ') bytes"
	@echo "Full uncompressed: $$(wc -c < $(WASM_DIR)/boilerplate-full.wasm | tr -d ' ') bytes"
	@echo "Full compressed:   $$(wc -c < $(WASM_DIR)/boilerplate-full.wasm.br | tr -d ' ') bytes"

copy-wasm-exec:
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/

wasm: compress-wasm copy-wasm-exec
	cp $(WASM_DIR)/wasm_exec.js $(WASM_DIR)/boilerplate.wasm $(WASM_DIR)/boilerplate.wasm.br $(WASM_DIR)/browser/
	cp $(WASM_DIR)/wasm_exec.js $(WASM_DIR)/boilerplate.wasm.br $(WASM_DIR)/node/
	cp $(WASM_DIR)/wasm_exec.js $(WASM_DIR)/boilerplate-full.wasm.br $(WASM_DIR)/node-full/
	@echo "WASM build complete:"
	@ls -lh $(WASM_DIR)/boilerplate.wasm $(WASM_DIR)/boilerplate.wasm.br $(WASM_DIR)/boilerplate-full.wasm $(WASM_DIR)/boilerplate-full.wasm.br

.PHONY: lint test default update-lint-config build-wasm build-wasm-lite build-wasm-full compress-wasm copy-wasm-exec wasm
