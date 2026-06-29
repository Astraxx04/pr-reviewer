#!/usr/bin/env sh
# prrev — one-liner installer for the PR Reviewer CLI.
# Usage: curl -fsSL https://raw.githubusercontent.com/Astraxx04/pr-reviewer/main/install.sh | sh
#
# Installs ONLY the prrev CLI. Override the location with INSTALL_DIR=... for a
# sudo-free, user-local install (e.g. INSTALL_DIR="$HOME/.local/bin").
set -e

REPO="https://github.com/Astraxx04/pr-reviewer"
BINARY="prrev"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Determine the latest release tag by following the /releases/latest redirect,
# which resolves to .../releases/tag/<tag>. This avoids the GitHub API (and its
# unauthenticated rate limit) and works without jq.
FINAL_URL="$(curl -fsSL -o /dev/null -w '%{url_effective}' "${REPO}/releases/latest")"
LATEST="${FINAL_URL##*/tag/}"
if [ -z "$LATEST" ] || [ "$LATEST" = "$FINAL_URL" ]; then
  echo "Could not determine latest release. Visit ${REPO}/releases to download manually." >&2
  exit 1
fi

VERSION="${LATEST#v}"
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="${REPO}/releases/download/${LATEST}/${ARCHIVE}"

echo "Downloading ${BINARY} ${LATEST} for ${OS}/${ARCH}..."
TMP="$(mktemp -d)"
curl -fsSL "$URL" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

# Elevate with sudo only if the install dir isn't writable (e.g. /usr/local/bin
# on macOS). Override with INSTALL_DIR=... for a sudo-free, user-local install.
mkdir -p "$INSTALL_DIR" 2>/dev/null || true
SUDO=""
if [ ! -w "$INSTALL_DIR" ]; then
  if command -v sudo >/dev/null 2>&1; then
    echo "Elevated permissions needed to write to ${INSTALL_DIR}; using sudo."
    SUDO="sudo"
  else
    echo "Error: ${INSTALL_DIR} is not writable and sudo is unavailable." >&2
    echo "Re-run with a writable dir, e.g.:" >&2
    echo "  curl -fsSL <url> | INSTALL_DIR=\"\$HOME/.local/bin\" sh" >&2
    rm -rf "$TMP"
    exit 1
  fi
fi
$SUDO mkdir -p "$INSTALL_DIR"

echo "Installing ${INSTALL_DIR}/${BINARY}"
$SUDO install -m 755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
rm -rf "$TMP"

echo ""
echo "Installed ${BINARY} ${LATEST}. Get started with:"
echo "  ${BINARY} auth login --server https://your-server"
echo ""
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "Note: ${INSTALL_DIR} is not on your PATH — add it to use '${BINARY}' directly." ;;
esac
