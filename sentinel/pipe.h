#pragma once

#include <windows.h>
#include <atomic>

// Pipe name for IPC.
constexpr const char* kPipeName = "\\\\.\\pipe\\kbdlock";

// Last time a PING was received (tick count in ms).
extern std::atomic<ULONGLONG> g_lastPingTime;

// Start the named pipe server on a new thread. Returns the thread handle.
HANDLE StartPipeServer();

// Signal the pipe server to stop.
void StopPipeServer();
