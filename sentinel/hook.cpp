#include "hook.h"

// ---------------------------------------------------------------------------
// Shared state
// ---------------------------------------------------------------------------

std::atomic<bool> g_locked{false};

static HHOOK s_hook = nullptr;

// Bitmap of currently pressed keys.
static bool s_keysDown[256] = {};

// Configurable unlock combo.
static int s_comboKeys[kMaxComboKeys] = {};
static int s_comboCount = 0;

// ESC hold tracking.
static ULONGLONG s_escDownSince = 0;

// Emergency combo: Ctrl+Shift+Alt+F12 (hardcoded, always available).
static bool IsEmergencyComboPressed() {
    return s_keysDown[VK_CONTROL] &&
           s_keysDown[VK_SHIFT] &&
           s_keysDown[VK_MENU] &&
           s_keysDown[VK_F12];
}

// Check if the configurable unlock combo is fully pressed.
static bool IsUnlockComboPressed() {
    if (s_comboCount < 2) return false;
    for (int i = 0; i < s_comboCount; i++) {
        if (!s_keysDown[s_comboKeys[i]]) return false;
    }
    return true;
}

// Check if ESC has been held long enough.
static bool IsEscHeldLongEnough() {
    if (!s_keysDown[VK_ESCAPE] || s_escDownSince == 0) return false;
    ULONGLONG elapsed = GetTickCount64() - s_escDownSince;
    return elapsed >= (kEscHoldSeconds * 1000ULL);
}

// Clear all key-down state (prevents stuck modifiers after unlock).
static void ClearKeyState() {
    for (int i = 0; i < 256; i++) {
        s_keysDown[i] = false;
    }
    s_escDownSince = 0;
}

static void DoUnlock() {
    g_locked.store(false, std::memory_order_release);
    ClearKeyState();
}

// ---------------------------------------------------------------------------
// Hook callback — must be fast (<50us)
// ---------------------------------------------------------------------------

static LRESULT CALLBACK LowLevelKeyboardProc(int nCode, WPARAM wParam, LPARAM lParam) {
    if (nCode != HC_ACTION) {
        return CallNextHookEx(s_hook, nCode, wParam, lParam);
    }

    auto* kb = reinterpret_cast<KBDLLHOOKSTRUCT*>(lParam);
    DWORD vk = kb->vkCode;
    if (vk >= 256) {
        return CallNextHookEx(s_hook, nCode, wParam, lParam);
    }

    bool keyDown = (wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN);
    bool keyUp   = (wParam == WM_KEYUP   || wParam == WM_SYSKEYUP);

    // Update key bitmap.
    if (keyDown) s_keysDown[vk] = true;
    if (keyUp)   s_keysDown[vk] = false;

    // Track ESC hold time.
    if (vk == VK_ESCAPE) {
        if (keyDown && s_escDownSince == 0) {
            s_escDownSince = GetTickCount64();
        } else if (keyUp) {
            s_escDownSince = 0;
        }
    }

    // If not locked, pass through.
    if (!g_locked.load(std::memory_order_acquire)) {
        return CallNextHookEx(s_hook, nCode, wParam, lParam);
    }

    // Check unlock conditions.
    if (IsEmergencyComboPressed() || IsUnlockComboPressed() || IsEscHeldLongEnough()) {
        DoUnlock();
        return CallNextHookEx(s_hook, nCode, wParam, lParam);
    }

    // Block the key.
    return 1;
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

bool InstallHook() {
    s_hook = SetWindowsHookExW(WH_KEYBOARD_LL, LowLevelKeyboardProc, nullptr, 0);
    return s_hook != nullptr;
}

void UninstallHook() {
    if (s_hook) {
        UnhookWindowsHookEx(s_hook);
        s_hook = nullptr;
    }
    ClearKeyState();
}

bool SetUnlockCombo(const int* vkCodes, int count) {
    if (count < 2 || count > kMaxComboKeys) return false;
    for (int i = 0; i < count; i++) {
        if (vkCodes[i] < 0 || vkCodes[i] > 255) return false;
        s_comboKeys[i] = vkCodes[i];
    }
    s_comboCount = count;
    return true;
}
