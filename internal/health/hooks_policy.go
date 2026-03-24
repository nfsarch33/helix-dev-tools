package health

import "strings"

// windowsHooksJSONPolicy enforces Windows-native hook commands for %USERPROFILE%\.cursor\hooks.json.
// Unix/WSL installs may keep ~/bin/cursor-tools (expanded by the user shell). On native Windows,
// tilde paths often force an extra cmd.exe layer and can flash a console per hook invocation.
func windowsHooksJSONPolicy(body string) (ok bool, detail string) {
	if strings.Contains(body, "~/bin/cursor-tools") {
		return false, "hooks.json uses ~/bin/cursor-tools; on Windows use absolute path to cursor-tools.exe (see sop/windows-cursor-hooks-console.md)"
	}
	if !strings.Contains(body, "cursor-tools") {
		return false, "hooks.json missing cursor-tools routes"
	}
	if strings.Contains(body, "cursor-tools.exe") {
		return true, ""
	}
	return false, "on Windows, prefer ...\\cursor-tools.exe with an absolute path (see sop/windows-cursor-hooks-console.md)"
}
