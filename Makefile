# Makefile for pi-game-of-life

APP_NAME = pi-game-of-life

# Default target
all: build

# Build native binary
build:
	go mod tidy
	go build -o $(APP_NAME)

# Run locally
run: build
	./$(APP_NAME)

# Build for WebAssembly
wasm:
	GOOS=js GOARCH=wasm go build -o main.wasm
	cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" .

# Serve the WASM build (requires Python 3)
serve:
	python3 -m http.server 8080

# Clean up build artifacts
clean:
	rm -f $(APP_NAME) main.wasm wasm_exec.js
