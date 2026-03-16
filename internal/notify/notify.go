package notify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/toast.v1"
)

const (
	appID               = "elliot40404.kbdlock"
	shortcutName        = "kbdlock.lnk"
	shortcutDescription = "kbdlock keyboard lock utility"
)

var (
	execCommand  = exec.Command
	executableFn = os.Executable
)

// EnsureReady creates the Start menu shortcut required for reliable desktop
// toast delivery on unpackaged Windows apps.
func EnsureReady() error {
	exePath, err := executableFn()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	shortcutPath, err := startMenuShortcutPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0o700); err != nil {
		return fmt.Errorf("create start menu directory: %w", err)
	}

	script := buildShortcutScript(shortcutPath, exePath, appID, shortcutDescription)
	cmd := execCommand("PowerShell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			return fmt.Errorf("create notification shortcut: %w", err)
		}
		return fmt.Errorf("create notification shortcut: %w: %s", err, msg)
	}

	return nil
}

// Notify shows a Windows toast notification.
func Notify(title, message string) error {
	n := toast.Notification{
		AppID:   appID,
		Title:   title,
		Message: message,
	}
	return n.Push()
}

func startMenuShortcutPath() (string, error) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return "", fmt.Errorf("APPDATA environment variable not set")
	}
	return filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", shortcutName), nil
}

func buildShortcutScript(shortcutPath, targetPath, appUserModelID, description string) string {
	return fmt.Sprintf(`
$shortcutPath = %s
$targetPath = %s
$appID = %s
$description = %s
$workingDirectory = Split-Path -Parent $targetPath
[System.IO.Directory]::CreateDirectory((Split-Path -Parent $shortcutPath)) | Out-Null

Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

[ComImport, Guid("00021401-0000-0000-C000-000000000046")]
public class CShellLink {}

[ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("000214F9-0000-0000-C000-000000000046")]
public interface IShellLinkW {
    void GetPath([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszFile, int cch, IntPtr pfd, uint fFlags);
    void GetIDList(out IntPtr ppidl);
    void SetIDList(IntPtr pidl);
    void GetDescription([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszName, int cch);
    void SetDescription([MarshalAs(UnmanagedType.LPWStr)] string pszName);
    void GetWorkingDirectory([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszDir, int cch);
    void SetWorkingDirectory([MarshalAs(UnmanagedType.LPWStr)] string pszDir);
    void GetArguments([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszArgs, int cch);
    void SetArguments([MarshalAs(UnmanagedType.LPWStr)] string pszArgs);
    void GetHotkey(out short pwHotkey);
    void SetHotkey(short wHotkey);
    void GetShowCmd(out int piShowCmd);
    void SetShowCmd(int iShowCmd);
    void GetIconLocation([Out, MarshalAs(UnmanagedType.LPWStr)] System.Text.StringBuilder pszIconPath, int cch, out int piIcon);
    void SetIconLocation([MarshalAs(UnmanagedType.LPWStr)] string pszIconPath, int iIcon);
    void SetRelativePath([MarshalAs(UnmanagedType.LPWStr)] string pszPathRel, uint dwReserved);
    void Resolve(IntPtr hwnd, uint fFlags);
    void SetPath([MarshalAs(UnmanagedType.LPWStr)] string pszFile);
}

[ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("0000010b-0000-0000-C000-000000000046")]
public interface IPersistFile {
    void GetClassID(out Guid pClassID);
    void IsDirty();
    void Load([MarshalAs(UnmanagedType.LPWStr)] string pszFileName, uint dwMode);
    void Save([MarshalAs(UnmanagedType.LPWStr)] string pszFileName, bool fRemember);
    void SaveCompleted([MarshalAs(UnmanagedType.LPWStr)] string pszFileName);
    void GetCurFile([MarshalAs(UnmanagedType.LPWStr)] out string ppszFileName);
}

[StructLayout(LayoutKind.Sequential, Pack = 4)]
public struct PROPERTYKEY {
    public Guid fmtid;
    public uint pid;
}

[StructLayout(LayoutKind.Explicit)]
public struct PROPVARIANT {
    [FieldOffset(0)] public ushort vt;
    [FieldOffset(8)] public IntPtr pwszVal;
}

[ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99")]
public interface IPropertyStore {
    void GetCount(out uint cProps);
    void GetAt(uint iProp, out PROPERTYKEY pkey);
    void GetValue(ref PROPERTYKEY key, out PROPVARIANT pv);
    void SetValue(ref PROPERTYKEY key, ref PROPVARIANT pv);
    void Commit();
}

public static class ShortcutHelper {
    private static readonly PROPERTYKEY AppUserModelIDKey = new PROPERTYKEY {
        fmtid = new Guid("9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3"),
        pid = 5,
    };

    public static void CreateShortcut(string shortcutPath, string targetPath, string workingDirectory, string description, string appUserModelID) {
        var link = (IShellLinkW)new CShellLink();
        link.SetPath(targetPath);
        link.SetWorkingDirectory(workingDirectory);
        link.SetDescription(description);
        link.SetIconLocation(targetPath, 0);

        var store = (IPropertyStore)link;
        var key = AppUserModelIDKey;
        var value = new PROPVARIANT {
            vt = 31,
            pwszVal = Marshal.StringToCoTaskMemUni(appUserModelID),
        };

        try {
            store.SetValue(ref key, ref value);
            store.Commit();
            ((IPersistFile)link).Save(shortcutPath, true);
        } finally {
            if (value.pwszVal != IntPtr.Zero) {
                Marshal.FreeCoTaskMem(value.pwszVal);
            }
        }
    }
}
"@

[ShortcutHelper]::CreateShortcut($shortcutPath, $targetPath, $workingDirectory, $description, $appID)
`, quotePowerShell(shortcutPath), quotePowerShell(targetPath), quotePowerShell(appUserModelID), quotePowerShell(description))
}

func quotePowerShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
