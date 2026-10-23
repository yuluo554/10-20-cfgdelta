# cfgdelta — build & verification entry points.
#
# Determinism / reproducibility knobs (plan/05 K-03):
#   CGO_ENABLED=0  → static binary, no libc runtime dependency
#   -trimpath      → no build-machine paths baked into the binary
#   -ldflags "-s -w" → strip symbols/debug info
VERSION := $(shell cat VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN := dist/cfgdelta$(shell go env GOEXE)

.PHONY: all build test vet gate gate-mutations privacy selftest check clean

all: check build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/cfgdelta

test:
	go test ./...

vet:
	go vet ./...

# Methodology layout gate: the first thing any new session runs.
gate:
	node tools/check-sprint-layout.mjs

# Mutation verification for the layout gate: proves each of its 8 criteria can
# actually go red. The layout gate's criteria count and this script's V-count
# must stay in step (verify-layout-mutations prints both).
gate-mutations:
	node tools/verify-layout-mutations.mjs

# Desensitization gate: no host paths / usernames / local proxies in the tree.
privacy:
	node tools/check-desensitized.mjs

selftest: build
	./$(BIN) selftest

# Everything a machine can check locally without network access.
check: gate gate-mutations privacy vet test
	node tools/sync-version.mjs --check
	node tools/sync-profiles.mjs --check
	node tools/gen-golden-manifest.mjs --check

clean:
	rm -rf dist
