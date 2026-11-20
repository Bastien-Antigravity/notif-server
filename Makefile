VERSION := $(strip $(shell cat VERSION.txt 2>/dev/null))
$(if $(VERSION),,$(error VERSION.txt is missing or empty))
LDFLAGS := -s -w -X 'github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/version.BuildVersion=$(VERSION)' -X 'github.com/Bastien-Antigravity/notif-server/src/core.ServerVersion=$(VERSION)'

.PHONY: all build build-lib core test verify-standalone version clean

all: build

version:
	@echo $(VERSION)

verify-standalone:
	@echo "Verifying standalone Go build (GOWORK=off)..."
	GOWORK=off go build ./...

build-lib core:
	@echo "Building CGO shared library libnotif (version $(VERSION))..."
	@mkdir -p notif/libnotif
	@if [ "$$(uname -s)" = "Darwin" ]; then \
		go build -ldflags="$(LDFLAGS)" -buildmode=c-shared -o notif/libnotif/libnotif.dylib ./cmd/libnotif; \
		install_name_tool -id @rpath/libnotif.dylib notif/libnotif/libnotif.dylib 2>/dev/null || true; \
	else \
		go build -ldflags="$(LDFLAGS)" -buildmode=c-shared -o notif/libnotif/libnotif.so ./cmd/libnotif; \
	fi

build: build-lib
	@echo "Building repository (version $(VERSION))..."
	@mkdir -p bin
	@if [ -f "go.mod" ]; then go build -ldflags="$(LDFLAGS)" -o bin/notif-server ./cmd/notif-server && go build -ldflags="$(LDFLAGS)" ./...; fi
	@if [ -f "Cargo.toml" ]; then cargo build --release; fi
	@if [ -f "setup.py" ] || [ -f "pyproject.toml" ]; then python3 -m build; fi

test:
	@echo "Running tests (version $(VERSION))..."
	@if [ -f "go.mod" ]; then go test -v ./...; fi
	@if [ -f "Cargo.toml" ]; then cargo test; fi
	@if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ]; then pytest; fi

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf dist build *.egg-info target/
	@rm -f notif/libnotif/*.dylib notif/libnotif/*.so notif/libnotif/*.dll notif/libnotif/*.h
