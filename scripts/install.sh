#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${HOME}/bin"
GO_VERSION="${GO_VERSION:-1.24.5}"
WITH_DEPS=false
DEPS_ONLY=false
CHECK_ONLY=false

usage() {
  cat <<'EOF'
Usage: scripts/install.sh [--with-deps] [--deps-only] [--check-only]

  --with-deps   Install missing WSL/Linux prerequisites before building cursor-tools
  --deps-only   Only install/check dependencies, do not build cursor-tools
  --check-only  Report dependency status without installing anything

Examples:
  scripts/install.sh
  scripts/install.sh --with-deps
  scripts/install.sh --with-deps --deps-only
  scripts/install.sh --check-only
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-deps)
      WITH_DEPS=true
      ;;
    --deps-only)
      WITH_DEPS=true
      DEPS_ONLY=true
      ;;
    --check-only)
      CHECK_ONLY=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[install] Unknown flag: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

log() {
  echo "[install] $*"
}

warn() {
  echo "[install] WARN: $*" >&2
}

fail() {
  echo "[install] ERROR: $*" >&2
  exit 1
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

is_wsl() {
  grep -qi microsoft /proc/version 2>/dev/null || [[ -n "${WSL_INTEROP:-}" || -n "${WSL_DISTRO_NAME:-}" ]]
}

ensure_sudo() {
  if ! have_cmd sudo; then
    fail "sudo is required for dependency installation"
  fi
}

ensure_apt_packages() {
  local missing=()
  local pkg
  for pkg in "$@"; do
    dpkg -s "$pkg" >/dev/null 2>&1 || missing+=("$pkg")
  done
  if [[ ${#missing[@]} -eq 0 ]]; then
    log "apt packages already installed: $*"
    return 0
  fi
  if [[ "$CHECK_ONLY" == true ]]; then
    warn "missing apt packages: ${missing[*]}"
    return 0
  fi
  ensure_sudo
  log "Installing apt packages: ${missing[*]}"
  sudo apt-get update
  sudo apt-get install -y "${missing[@]}"
}

install_uv() {
  if have_cmd uv; then
    log "uv already installed: $(uv --version 2>/dev/null | head -n 1)"
    return 0
  fi
  if [[ "$CHECK_ONLY" == true ]]; then
    warn "uv missing"
    return 0
  fi
  have_cmd curl || fail "curl is required to install uv"
  log "Installing uv"
  curl -LsSf https://astral.sh/uv/install.sh | sh
}

install_go() {
  if have_cmd go && go version | grep -Eq 'go1\.(24|2[5-9]|[3-9][0-9])'; then
    log "Go already installed: $(go version)"
    return 0
  fi
  if [[ "$CHECK_ONLY" == true ]]; then
    warn "Go 1.24+ missing"
    return 0
  fi
  have_cmd curl || fail "curl is required to install Go"
  have_cmd tar || fail "tar is required to install Go"
  ensure_sudo

  local archive="go${GO_VERSION}.linux-amd64.tar.gz"
  local url="https://go.dev/dl/${archive}"
  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' RETURN

  log "Installing Go ${GO_VERSION}"
  curl -fsSL "$url" -o "${tmp_dir}/${archive}"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "${tmp_dir}/${archive}"

  case ":${PATH}:" in
    *":/usr/local/go/bin:"*) ;;
    *)
      warn "Add /usr/local/go/bin to PATH if it is not already present in your shell profile"
      ;;
  esac
}

install_gh() {
  if have_cmd gh; then
    log "gh already installed: $(gh --version 2>/dev/null | head -n 1)"
    return 0
  fi
  if [[ "$CHECK_ONLY" == true ]]; then
    warn "gh missing"
    return 0
  fi
  ensure_apt_packages gh
}

install_rtk() {
  if have_cmd rtk; then
    log "rtk already installed: $(rtk --version 2>/dev/null | head -n 1)"
    return 0
  fi
  if [[ "$CHECK_ONLY" == true ]]; then
    warn "rtk missing"
    return 0
  fi
  have_cmd curl || fail "curl is required to install rtk"
  log "Installing rtk"
  curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh
}

check_optional_tool() {
  local name="$1"
  if have_cmd "$name"; then
    log "$name already installed"
  else
    warn "$name not installed (optional)"
  fi
}

install_prereqs() {
  if [[ "$(uname -s)" != "Linux" ]]; then
    warn "Automatic dependency installation currently targets Linux/WSL. Run 'cursor-tools doctor deps' for a checklist on this platform."
    return 0
  fi

  if ! is_wsl; then
    warn "Proceeding on Linux outside WSL; Docker/NVIDIA setup may differ from the WSL baseline."
  fi

  ensure_apt_packages git openssh-client nodejs npm python3 jq curl make build-essential
  install_uv
  install_go
  install_gh
  install_rtk

  if have_cmd docker; then
    log "docker already installed: $(docker --version 2>/dev/null | head -n 1)"
  else
    warn "docker missing; install Docker Desktop with WSL integration or docker-ce before Mission Control bring-up"
  fi

  if have_cmd nvidia-smi; then
    log "nvidia-smi already installed"
  else
    warn "nvidia-smi missing; verify the Windows NVIDIA driver and WSL GPU passthrough before GPU workloads"
  fi

  check_optional_tool kubectl
  check_optional_tool terraform
  check_optional_tool helm
}

if [[ "$WITH_DEPS" == true || "$CHECK_ONLY" == true ]]; then
  install_prereqs
fi

if [[ "$DEPS_ONLY" == true || "$CHECK_ONLY" == true ]]; then
  if [[ -x "${BIN_DIR}/cursor-tools" ]]; then
    log "Running dependency verification"
    "${BIN_DIR}/cursor-tools" doctor deps || true
  else
    warn "cursor-tools binary not installed yet; skip doctor deps verification"
  fi
  exit 0
fi

have_cmd go || fail "Go 1.24+ is required to build cursor-tools"

log "Building cursor-tools..."
cd "$REPO_ROOT"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o bin/cursor-tools ./cmd/cursor-tools/

mkdir -p "$BIN_DIR"
cp bin/cursor-tools "$BIN_DIR/cursor-tools"
chmod +x "$BIN_DIR/cursor-tools"

log "Installed cursor-tools ${VERSION} to ${BIN_DIR}/cursor-tools"
log "Verify: ${BIN_DIR}/cursor-tools doctor deps"
