#!/usr/bin/env sh
# PR Reviewer — one-liner installer
# Usage: curl -fsSL https://raw.githubusercontent.com/<owner>/pr-reviewer/main/install.sh | sh
set -e

REPO="https://github.com/Astraxx04/pr-reviewer"
BINARY="pr-reviewer"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Fetch the latest release tag.
LATEST="$(curl -fsSL "${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')"
if [ -z "$LATEST" ]; then
  echo "Could not determine latest release. Visit ${REPO}/releases to download manually." >&2
  exit 1
fi

VERSION="${LATEST#v}"
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="${REPO}/releases/download/${LATEST}/${ARCHIVE}"

echo "Downloading pr-reviewer ${LATEST} for ${OS}/${ARCH}..."
TMP="$(mktemp -d)"
curl -fsSL "$URL" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

# Install every binary bundled in the archive (server, migrate, cli=prrev).
for bin in pr-reviewer pr-reviewer-migrate prrev; do
  if [ -f "${TMP}/${bin}" ]; then
    echo "Installing ${INSTALL_DIR}/${bin}"
    install -m 755 "${TMP}/${bin}" "${INSTALL_DIR}/${bin}"
  fi
done
rm -rf "$TMP"

echo ""
echo "Installation complete."
echo "  Server:  ${BINARY} --help"
echo "  CLI:     prrev --help"
