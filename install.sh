#!/bin/sh
# TagIt installer
# Usage: curl -fsSL https://raw.githubusercontent.com/liliang-cn/tagit/main/install.sh | sh
# Or:    curl -fsSL https://raw.githubusercontent.com/liliang-cn/tagit/main/install.sh | INSTALL_DIR=/usr/local/bin sh

set -e

REPO="liliang-cn/tagit"
BINARIES="tagit tagitd"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
MIN_GO_VERSION="1.22"

# ── helpers ─────────────────────────────────────────────────────────────────

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m ok\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33mwarn\033[0m %s\n' "$*" >&2; }
die()   { printf '\033[1;31merr\033[0m %s\n' "$*" >&2; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }

# ── platform detection ───────────────────────────────────────────────────────

detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux)  OS="linux" ;;
        Darwin) OS="darwin" ;;
        *)      die "unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64)          ARCH="amd64" ;;
        aarch64 | arm64) ARCH="arm64" ;;
        *)               die "unsupported architecture: $ARCH" ;;
    esac
}

# ── go version check ─────────────────────────────────────────────────────────

go_version_ok() {
    command -v go >/dev/null 2>&1 || return 1
    # parse "go1.24.2" → major.minor
    ver="$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')"
    major="$(echo "$ver" | cut -d. -f1)"
    minor="$(echo "$ver" | cut -d. -f2)"
    need_major="$(echo "$MIN_GO_VERSION" | cut -d. -f1)"
    need_minor="$(echo "$MIN_GO_VERSION" | cut -d. -f2)"
    [ "$major" -gt "$need_major" ] && return 0
    [ "$major" -eq "$need_major" ] && [ "$minor" -ge "$need_minor" ] && return 0
    return 1
}

# ── install via go install ───────────────────────────────────────────────────

install_with_go() {
    info "Go $(go version | awk '{print $3}') found — building from source"
    GOBIN="$INSTALL_DIR" go install "github.com/$REPO/cmd/tagit@latest"
    GOBIN="$INSTALL_DIR" go install "github.com/$REPO/cmd/tagitd@latest"
}

# ── install prebuilt binary ──────────────────────────────────────────────────

install_prebuilt() {
    need_cmd curl
    need_cmd tar

    info "Fetching latest release tag…"
    TAG="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
    [ -n "$TAG" ] || die "could not determine latest release tag (are releases published?)"

    ARCHIVE="tagit_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/$REPO/releases/download/$TAG/$ARCHIVE"

    info "Downloading $TAG for ${OS}/${ARCH}…"
    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR"' EXIT

    curl -fsSL "$URL" -o "$TMPDIR/$ARCHIVE" || die "download failed: $URL"
    tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

    for bin in $BINARIES; do
        src="$TMPDIR/$bin"
        [ -f "$src" ] || src="$TMPDIR/${OS}_${ARCH}/$bin"
        [ -f "$src" ] || die "binary '$bin' not found in archive"
        install -m 0755 "$src" "$INSTALL_DIR/$bin"
    done
}

# ── PATH check ───────────────────────────────────────────────────────────────

check_path() {
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) return 0 ;;
    esac

    SHELL_NAME="$(basename "${SHELL:-sh}")"
    case "$SHELL_NAME" in
        zsh)  RC="$HOME/.zshrc" ;;
        bash) RC="$HOME/.bashrc" ;;
        *)    RC="$HOME/.profile" ;;
    esac

    warn "$INSTALL_DIR is not in PATH"
    warn "Add it by running:"
    warn "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> $RC && source $RC"
}

# ── main ─────────────────────────────────────────────────────────────────────

verify_install() {
    for bin in $BINARIES; do
        BIN_PATH="$INSTALL_DIR/$bin"
        [ -x "$BIN_PATH" ] || die "installation failed: $BIN_PATH not found or not executable"
        ok "verified $BIN_PATH"
    done
    # confirm tagit CLI responds correctly
    "$INSTALL_DIR/tagit" --help >/dev/null 2>&1 || die "tagit --help failed — binary may be corrupt"
    ok "tagit --help: OK"
}

main() {
    detect_platform
    info "Platform: ${OS}/${ARCH}"

    mkdir -p "$INSTALL_DIR" || die "cannot create install dir: $INSTALL_DIR"

    # create TagIt home directory and required subdirectories
    mkdir -p "$HOME/.tagit"
    ok "created $HOME/.tagit"

    if go_version_ok; then
        install_with_go
    else
        if command -v go >/dev/null 2>&1; then
            warn "Go found but version is below $MIN_GO_VERSION — falling back to prebuilt binary"
        else
            info "Go not found — downloading prebuilt binary"
        fi
        install_prebuilt
    fi

    verify_install
    check_path

    printf '\n'
    info "TagIt installed. Next steps:"
    printf '  1. Register an agent (any installed CLI works; claude and codex shown):\n'
    printf '       tagit agent add claude "Claude" $(which claude)\n'
    printf '       tagit agent add codex  "Codex"  $(which codex)\n'
    printf '\n'
    printf '  2. Start the daemon:\n'
    printf '       tagit start\n'
    printf '\n'
    printf '  3. Run a task:\n'
    printf '       tagit run --agent claude "your task here"\n'
    printf '\n'
    printf '  4. Stop the daemon:\n'
    printf '       tagit stop\n'
}

main "$@"
