#include "pipe.h"
#include "hook.h"

#include <cstdio>
#include <cstring>
#include <cstdlib>

// ---------------------------------------------------------------------------
// Shared state
// ---------------------------------------------------------------------------

std::atomic<ULONGLONG> g_lastPingTime{0};
static std::atomic<bool> s_pipeRunning{true};

// ---------------------------------------------------------------------------
// Command parsing
// ---------------------------------------------------------------------------

// Parse a "SET_COMBO <vk1> <vk2> ..." command. Returns true on success.
static bool HandleSetCombo(const char* args) {
    int codes[kMaxComboKeys];
    int count = 0;
    const char* p = args;

    while (*p && count < kMaxComboKeys) {
        // Skip whitespace.
        while (*p == ' ') p++;
        if (*p == '\0') break;

        char* end = nullptr;
        long val = strtol(p, &end, 10);
        if (end == p || val < 0 || val > 255) return false;
        codes[count++] = static_cast<int>(val);
        p = end;
    }

    return SetUnlockCombo(codes, count);
}

// Process a single command line. Writes response into buf.
static void ProcessCommand(const char* cmd, char* response, int responseSize) {
    if (strcmp(cmd, "LOCK") == 0) {
        g_locked.store(true, std::memory_order_release);
        g_lastPingTime.store(GetTickCount64(), std::memory_order_release);
        strncpy(response, "OK\n", responseSize);
    } else if (strcmp(cmd, "UNLOCK") == 0) {
        g_locked.store(false, std::memory_order_release);
        strncpy(response, "OK\n", responseSize);
    } else if (strcmp(cmd, "PING") == 0) {
        g_lastPingTime.store(GetTickCount64(), std::memory_order_release);
        strncpy(response, "PONG\n", responseSize);
    } else if (strcmp(cmd, "STATUS") == 0) {
        bool locked = g_locked.load(std::memory_order_acquire);
        strncpy(response, locked ? "LOCKED\n" : "UNLOCKED\n", responseSize);
    } else if (strncmp(cmd, "SET_COMBO ", 10) == 0) {
        bool ok = HandleSetCombo(cmd + 10);
        strncpy(response, ok ? "OK\n" : "ERROR\n", responseSize);
    } else if (strcmp(cmd, "QUIT") == 0) {
        strncpy(response, "OK\n", responseSize);
        PostQuitMessage(0);
    } else {
        strncpy(response, "ERROR\n", responseSize);
    }
    response[responseSize - 1] = '\0';
}

// ---------------------------------------------------------------------------
// Pipe server thread
// ---------------------------------------------------------------------------

static DWORD WINAPI PipeServerThread(LPVOID /*param*/) {
    while (s_pipeRunning.load(std::memory_order_acquire)) {
        HANDLE hPipe = CreateNamedPipeA(
            kPipeName,
            PIPE_ACCESS_DUPLEX,
            PIPE_TYPE_MESSAGE | PIPE_READMODE_MESSAGE | PIPE_WAIT,
            1,     // single instance
            512,   // output buffer
            512,   // input buffer
            1000,  // default timeout ms
            nullptr // default security (same user)
        );

        if (hPipe == INVALID_HANDLE_VALUE) {
            Sleep(1000);
            continue;
        }

        // Wait for client. Use overlapped so we can check s_pipeRunning.
        OVERLAPPED ov = {};
        ov.hEvent = CreateEventW(nullptr, TRUE, FALSE, nullptr);
        ConnectNamedPipe(hPipe, &ov);

        // Wait with timeout so we can exit.
        while (s_pipeRunning.load(std::memory_order_acquire)) {
            DWORD result = WaitForSingleObject(ov.hEvent, 500);
            if (result == WAIT_OBJECT_0) break;
        }
        CloseHandle(ov.hEvent);

        if (!s_pipeRunning.load(std::memory_order_acquire)) {
            CloseHandle(hPipe);
            break;
        }

        // Read commands until client disconnects.
        char buf[512];
        DWORD bytesRead;
        while (s_pipeRunning.load(std::memory_order_acquire)) {
            BOOL ok = ReadFile(hPipe, buf, sizeof(buf) - 1, &bytesRead, nullptr);
            if (!ok || bytesRead == 0) break;

            buf[bytesRead] = '\0';

            // Strip trailing newline/CR.
            while (bytesRead > 0 && (buf[bytesRead-1] == '\n' || buf[bytesRead-1] == '\r')) {
                buf[--bytesRead] = '\0';
            }

            char response[256];
            ProcessCommand(buf, response, sizeof(response));

            DWORD bytesWritten;
            WriteFile(hPipe, response, (DWORD)strlen(response), &bytesWritten, nullptr);

            // If QUIT was sent, break out.
            if (strcmp(buf, "QUIT") == 0) break;
        }

        DisconnectNamedPipe(hPipe);
        CloseHandle(hPipe);
    }
    return 0;
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

HANDLE StartPipeServer() {
    s_pipeRunning.store(true, std::memory_order_release);
    return CreateThread(nullptr, 0, PipeServerThread, nullptr, 0, nullptr);
}

void StopPipeServer() {
    s_pipeRunning.store(false, std::memory_order_release);
}
