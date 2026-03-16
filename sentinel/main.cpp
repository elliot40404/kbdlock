#include "hook.h"
#include "pipe.h"

#include <cstdio>

// Heartbeat check interval (ms).
constexpr UINT kHeartbeatTimerMs = 5000;
// Max time without ping before auto-unlock (ms).
constexpr ULONGLONG kHeartbeatTimeoutMs = 60000;

static constexpr UINT_PTR kHeartbeatTimerId = 1;

// ---------------------------------------------------------------------------
// Heartbeat timer callback
// ---------------------------------------------------------------------------

static void CALLBACK HeartbeatTimerProc(HWND, UINT, UINT_PTR, DWORD) {
    if (!g_locked.load(std::memory_order_acquire)) return;

    ULONGLONG lastPing = g_lastPingTime.load(std::memory_order_acquire);
    if (lastPing == 0) return;

    ULONGLONG now = GetTickCount64();
    if (now - lastPing > kHeartbeatTimeoutMs) {
        g_locked.store(false, std::memory_order_release);
    }
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

int WINAPI WinMain(HINSTANCE, HINSTANCE, LPSTR, int) {
    // Prevent multiple instances.
    HANDLE mutex = CreateMutexW(nullptr, TRUE, L"Global\\kbdlock-hook-mutex");
    if (GetLastError() == ERROR_ALREADY_EXISTS) {
        return 1;
    }

    // Start pipe server thread.
    HANDLE pipeThread = StartPipeServer();
    if (!pipeThread) {
        ReleaseMutex(mutex);
        CloseHandle(mutex);
        return 1;
    }

    // Install keyboard hook.
    if (!InstallHook()) {
        StopPipeServer();
        WaitForSingleObject(pipeThread, 3000);
        CloseHandle(pipeThread);
        ReleaseMutex(mutex);
        CloseHandle(mutex);
        return 1;
    }

    // Start heartbeat timer.
    SetTimer(nullptr, kHeartbeatTimerId, kHeartbeatTimerMs, HeartbeatTimerProc);

    // Message loop (required for low-level hook to work).
    MSG msg;
    while (GetMessage(&msg, nullptr, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessage(&msg);
    }

    // Cleanup.
    KillTimer(nullptr, kHeartbeatTimerId);
    UninstallHook();
    StopPipeServer();
    WaitForSingleObject(pipeThread, 3000);
    CloseHandle(pipeThread);
    ReleaseMutex(mutex);
    CloseHandle(mutex);

    return 0;
}
