package assets

import _ "embed"

//go:embed locked.ico
var LockedIcon []byte

//go:embed unlocked.ico
var UnlockedIcon []byte

//go:embed kbdlock-hook.exe
var SentinelExe []byte
