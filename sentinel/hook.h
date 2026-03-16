#pragma once

#include <windows.h>
#include <atomic>

// Maximum keys in an unlock combo.
constexpr int kMaxComboKeys = 8;

// Install/uninstall the low-level keyboard hook.
bool InstallHook();
void UninstallHook();

// Set the configurable unlock combo (VK codes). Returns false if count is invalid.
bool SetUnlockCombo(const int* vkCodes, int count);

// Lock/unlock state (shared with pipe thread via atomics).
extern std::atomic<bool> g_locked;

// ESC hold duration in seconds before emergency unlock.
constexpr int kEscHoldSeconds = 5;
