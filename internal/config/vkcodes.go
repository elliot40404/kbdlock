package config

import (
	"fmt"
	"strings"
)

// vkCodeMap maps key names to Windows virtual key codes.
var vkCodeMap = map[string]uint32{
	// Modifiers
	"CTRL": 0xA2, "LCTRL": 0xA2, "RCTRL": 0xA3,
	"ALT": 0xA4, "LALT": 0xA4, "RALT": 0xA5,
	"SHIFT": 0xA0, "LSHIFT": 0xA0, "RSHIFT": 0xA1,

	// Function keys
	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73,
	"F5": 0x74, "F6": 0x75, "F7": 0x76, "F8": 0x77,
	"F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,

	// Letters
	"A": 0x41, "B": 0x42, "C": 0x43, "D": 0x44, "E": 0x45,
	"F": 0x46, "G": 0x47, "H": 0x48, "I": 0x49, "J": 0x4A,
	"K": 0x4B, "L": 0x4C, "M": 0x4D, "N": 0x4E, "O": 0x4F,
	"P": 0x50, "Q": 0x51, "R": 0x52, "S": 0x53, "T": 0x54,
	"U": 0x55, "V": 0x56, "W": 0x57, "X": 0x58, "Y": 0x59,
	"Z": 0x5A,

	// Special
	"ESC": 0x1B, "ESCAPE": 0x1B,
	"TAB": 0x09, "SPACE": 0x20, "ENTER": 0x0D,
	"DELETE": 0x2E, "BACKSPACE": 0x08,
}

// ComboToVKCodes converts a slice of key names to their VK code values.
func ComboToVKCodes(names []string) ([]uint32, error) {
	codes := make([]uint32, 0, len(names))
	for _, name := range names {
		vk, ok := vkCodeMap[strings.ToUpper(strings.TrimSpace(name))]
		if !ok {
			return nil, fmt.Errorf("unknown key name: %q", name)
		}
		codes = append(codes, vk)
	}
	return codes, nil
}

// modifierNames maps key names to RegisterHotKey modifier flags.
var modifierNames = map[string]uint32{
	"CTRL": 0x0002, "LCTRL": 0x0002, "RCTRL": 0x0002,
	"ALT": 0x0001, "LALT": 0x0001, "RALT": 0x0001,
	"SHIFT": 0x0004, "LSHIFT": 0x0004, "RSHIFT": 0x0004,
}

// SplitHotkeyCombo splits a key name slice into RegisterHotKey modifiers and
// a single virtual-key code. It returns an error if there are zero or more
// than one non-modifier keys.
func SplitHotkeyCombo(names []string) (modifiers, vk uint32, err error) {
	var nonMod []string
	for _, name := range names {
		upper := strings.ToUpper(strings.TrimSpace(name))
		if mod, ok := modifierNames[upper]; ok {
			modifiers |= mod
		} else {
			nonMod = append(nonMod, upper)
		}
	}
	if len(nonMod) != 1 {
		return 0, 0, fmt.Errorf("hotkey must have exactly 1 non-modifier key, got %d", len(nonMod))
	}
	code, ok := vkCodeMap[nonMod[0]]
	if !ok {
		return 0, 0, fmt.Errorf("unknown key name: %q", nonMod[0])
	}
	return modifiers, code, nil
}
