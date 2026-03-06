#!/bin/bash
# Git hook shim: delegates to cursor-tools binary.
# Usage: symlink or copy this as commit-msg / pre-push in git-hooks/.
HOOK_NAME=$(basename "$0")
exec cursor-tools githook "$HOOK_NAME" "$@"
