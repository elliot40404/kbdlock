default: all

version := env("VERSION", "dev")

all: sentinel controller

sentinel:
    @mkdir -p build
    g++ -O2 -Wall -Wextra -mwindows -static -D_WIN32_WINNT=0x0600 \
        sentinel/main.cpp sentinel/hook.cpp sentinel/pipe.cpp \
        -o build/kbdlock-hook.exe -luser32
    cp build/kbdlock-hook.exe assets/kbdlock-hook.exe

controller: sentinel
    @mkdir -p build
    go build -ldflags "-X main.version={{version}}" -o build/kbdlock.exe .

clean:
    rm -rf build/
    rm -f assets/kbdlock-hook.exe

lint:
    golangci-lint run --fix

fix-fmt:
    @echo "=> Modernizing code (go fix)..."
    go fix ./...
    @echo "=> Formatting code (gofumpt)..."
    gofumpt -l -w .
    @echo "=> Fixing imports (goimports)..."
    goimports -w .

verify: fix-fmt lint all

vendor:
    go mod tidy
    go mod vendor
    go mod tidy
