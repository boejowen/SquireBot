//go:build windows

package eqfind

import (
	"regexp"

	"golang.org/x/sys/windows/registry"
)

// scanUninstallKeys enumerates Windows uninstall registry hives looking for
// "Project 1999" / "EverQuest" / "Sony EverQuest" entries with an
// InstallLocation that ValidateFolder accepts.
//
// Hives probed (per RESEARCH.md §6.5):
//   - HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall
//   - HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall
//   - HKLM\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall   (32-bit on 64-bit OS)
//
// Threat T-04-02: registry-supplied InstallLocation is treated as
// untrusted-but-validated. ValidateFolder is the gate; it requires both
// eqgame.exe AND eqclient.ini, which neither System32 nor a path-traversed
// arbitrary directory can provide.
func scanUninstallKeys() string {
	displayNameRe := regexp.MustCompile(`(?i)^(Project 1999|EverQuest|Sony EverQuest)\b`)

	hives := []struct {
		root   registry.Key
		path   string
		access uint32
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.ENUMERATE_SUB_KEYS | registry.QUERY_VALUE},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.ENUMERATE_SUB_KEYS | registry.QUERY_VALUE},
		// WOW6432Node — 32-bit hive on 64-bit OS. Use WOW64_32KEY access flag instead
		// of literal path so the redirector hands us the right view regardless of
		// process bitness.
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, registry.ENUMERATE_SUB_KEYS | registry.QUERY_VALUE | registry.WOW64_32KEY},
	}

	for _, hive := range hives {
		k, err := registry.OpenKey(hive.root, hive.path, hive.access)
		if err != nil {
			continue // hive missing or access denied; try the next
		}
		names, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}
		for _, name := range names {
			sub, err := registry.OpenKey(hive.root, hive.path+`\`+name, registry.QUERY_VALUE|(hive.access&registry.WOW64_32KEY))
			if err != nil {
				continue
			}
			displayName, _, err := sub.GetStringValue("DisplayName")
			if err != nil {
				sub.Close()
				continue
			}
			if !displayNameRe.MatchString(displayName) {
				sub.Close()
				continue
			}
			installLoc, _, err := sub.GetStringValue("InstallLocation")
			sub.Close()
			if err != nil || installLoc == "" {
				continue
			}
			// Validate — neither System32 nor an attacker-redirected path will
			// have BOTH eqgame.exe AND eqclient.ini, so ValidateFolder is the
			// path-traversal gate per T-04-02.
			if ValidateFolder(installLoc) == nil {
				return installLoc
			}
		}
	}
	return ""
}
