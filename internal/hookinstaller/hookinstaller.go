// Package hookinstaller writes git hook scripts for helix-dev-tools integration.
package hookinstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const prePushTmpl = `#!/usr/bin/env sh
# Installed by {{.Binary}} install hooks
# Runs rebrand scan before push to nfsarch33/* repos.
set -e

remote="$2"

if echo "$remote" | grep -q "nfsarch33"; then
  result=$({{.Binary}} rebrand scan --dir . --format json 2>&1)
  count=$(echo "$result" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' | head -1)
  count=${count:-0}
  if [ "$count" -gt 0 ]; then
    echo "[rebrand-pre-push] Legacy terms found ($count). Fix or add to .rebrand-allowlist.yaml." >&2
    echo "$result" >&2
    exit 1
  fi
fi
`

type hookData struct {
	Binary string
}

// InstallPrePushHook writes a pre-push shell hook to <repoDir>/.git/hooks/pre-push.
// The hook invokes <binary> rebrand scan --dir . and blocks the push when legacy
// terms are found. Creates .git/hooks if it does not exist.
func InstallPrePushHook(repoDir, binary string) error {
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-push")
	f, err := os.OpenFile(hookPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("open hooks/pre-push: %w", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("pre-push").Parse(prePushTmpl))
	return tmpl.Execute(f, hookData{Binary: binary})
}
