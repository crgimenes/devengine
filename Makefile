# devengine — library + CLI tools under cmd/
#
# Targets:
#   make build       — build every cmd/<name> for the current OS/arch into bin/
#   make build-cross — also build for linux/amd64 and freebsd/amd64
#   make check       — go fix + go vet + go test (verification flow)
#   make security    — gosec ./...
#   make tidy        — go mod tidy
#   make clean       — remove bin/
#   make clean-all   — also remove dev DB files and session blobs

export CGO_ENABLED=0
BUILD_FLAGS := -trimpath -ldflags "-s -w -extldflags '-static -w'"

CMDS   := $(notdir $(wildcard cmd/*))
GOOS   := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

.PHONY: all build build-cross check fix vet test security tidy clean clean-all

all: build

build:
	@mkdir -p bin
	@for cmd in $(CMDS); do \
	  echo "  -> $$cmd ($(GOOS)-$(GOARCH))"; \
	  go build $(BUILD_FLAGS) -o bin/$$cmd-$(GOOS)-$(GOARCH) ./cmd/$$cmd ; \
	done

build-cross: build
	@for cmd in $(CMDS); do \
	  for target in linux-amd64 freebsd-amd64; do \
	    os=$${target%-*}; arch=$${target#*-}; \
	    echo "  -> $$cmd ($$os-$$arch)"; \
	    GOOS=$$os GOARCH=$$arch go build $(BUILD_FLAGS) -o bin/$$cmd-$$os-$$arch ./cmd/$$cmd ; \
	  done ; \
	done

check: fix vet test

fix:
	go fix ./...
	go fix -inline ./...

vet:
	go vet ./...

test:
	go test ./...

security:
	gosec ./...

tidy:
	go mod tidy

clean:
	rm -rf bin

clean-all: clean
	rm -f *.db-shm *.db-wal *.db sessions.gob
