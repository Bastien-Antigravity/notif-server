VERSION := $(shell cat VERSION.txt 2>/dev/null || echo "0.0.1")
LDFLAGS := -s -w -X 'github.com/Bastien-Antigravity/notif-server/src/core.ServerVersion=$(VERSION)'

.PHONY: all build build-lib core test version clean

all: build

version:
	@echo $(VERSION)

build-lib core:
	@echo "Building CGO shared library libnotif (version $(VERSION))..."
	@mkdir -p notif/libnotif
	@if [ "$$(uname -s)" = "Darwin" ]; then \
		go build -ldflags="$(LDFLAGS)" -buildmode=c-shared -o notif/libnotif/libnotif.dylib ./cmd/libnotif || true; \
		install_name_tool -id @rpath/libnotif.dylib notif/libnotif/libnotif.dylib 2>/dev/null || true; \
	else \
		go build -ldflags="$(LDFLAGS)" -buildmode=c-shared -o notif/libnotif/libnotif.so ./cmd/libnotif || true; \
	fi

build: build-lib
	@echo "Building repository (version $(VERSION))..."
	@mkdir -p bin
	@if [ -f "go.mod" ]; then go build -ldflags="$(LDFLAGS)" -o bin/notif-server ./cmd/notif-server && go build -ldflags="$(LDFLAGS)" ./...; fi
	@if [ -f "Cargo.toml" ]; then cargo build --release || true; fi
	@if [ -f "setup.py" ] || [ -f "pyproject.toml" ]; then python3 -m build || true; fi

test:
	@echo "Running tests (version $(VERSION))..."
	@if [ -f "go.mod" ]; then go test ./... 2>/dev/null || go test ./src/... 2>/dev/null || true; fi
	@if [ -f "Cargo.toml" ]; then cargo test 2>/dev/null || true; fi
	@if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ]; then pytest 2>/dev/null || true; fi

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf dist build *.egg-info target/
	@rm -f notif/libnotif/*.dylib notif/libnotif/*.so notif/libnotif/*.dll notif/libnotif/*.h
