#!/bin/sh
##[>] 🤖🤖
set -eu

SCRIPT="$(dirname "$0")/../che/che-install.sh"
RELEASES_API='https://gitlab.com/api/v4/projects/konradodwrot%2Fgo-modules/packages?package_name=che&package_type=generic&order_by=created_at&sort=desc&per_page=20'

if command -v curl >/dev/null 2>&1; then
  fetch_stdout() { curl -fsS "$1"; }
else
  fetch_stdout() { wget -q -O - "$1"; }
fi

released() {
  fetch_stdout "$RELEASES_API" | grep -oE '"version":"[0-9]+\.[0-9]+\.[0-9]+"' | cut -d'"' -f4 | head -2
}
if [ -z "${1:-}" ] || [ -z "${2:-}" ]; then
  set -- $(released)
  [ $# -eq 2 ] || { echo "no released che versions" >&2; exit 1; }
  set -- "$2" "$1"
fi
OLD="$1"
NEW="$2"
echo "old=$OLD new=$NEW"

CHE_INSTALL_DIR="$(mktemp -d)"
export CHE_INSTALL_DIR
export PATH="${CHE_INSTALL_DIR}:${PATH}"
trap 'rm -rf "$CHE_INSTALL_DIR"' EXIT

SHELLS=sh
if command -v dash >/dev/null 2>&1; then SHELLS="sh dash"; fi

sh -n "$SCRIPT"

step() {
  want="$1"
  shift
  if ! out="$("$@" 2>&1)"; then
    printf 'FAIL: %s\n  exit non-zero: %s\n' "$*" "$out" >&2
    exit 1
  fi
  case "$out" in
    *"$want"*) echo "ok: $out" ;;
    *) printf 'FAIL: %s\n  want: %s\n  got:  %s\n' "$*" "$want" "$out" >&2; exit 1 ;;
  esac
}

for shell in $SHELLS; do
  echo "== $shell"
  rm -f "${CHE_INSTALL_DIR}/che"
  if [ "${LATEST_FALLBACK:-0}" = 1 ]; then
    step "installed che $NEW" env CHE_VERSION="$NEW" "$shell" "$SCRIPT"
  else
    step "installed che" env -u CHE_VERSION "$shell" "$SCRIPT"
  fi
  step "installed che $OLD" "$shell" "$SCRIPT" --version "$OLD"
  step "skip ($OLD installed, wanted $OLD)" env CHE_VERSION="$OLD" "$shell" "$SCRIPT"
  step "skip (present)" env CHE_VERSION="$NEW" "$shell" "$SCRIPT" --skip-if-present
  step "installed che $NEW" env CHE_VERSION="$NEW" "$shell" "$SCRIPT" --skip-if-present-is-newer
  step "skip ($NEW installed, wanted $OLD)" env CHE_VERSION="$OLD" "$shell" "$SCRIPT" --skip-if-present-is-newer
  step "skip ($NEW installed, wanted $NEW)" env CHE_VERSION="$OLD" "$shell" "$SCRIPT" --version "$NEW"
  step "skip (present)" env CHE_VERSION="$OLD" CHE_SKIP_IF_PRESENT=1 "$shell" "$SCRIPT"
  step "skip ($NEW installed, wanted $OLD)" env CHE_VERSION="$OLD" CHE_SKIP_IF_PRESENT_IS_NEWER=1 "$shell" "$SCRIPT"
  step "installed che $OLD" env -u CHE_INSTALL_DIR "$shell" "$SCRIPT" --version "$OLD" --install-dir "$CHE_INSTALL_DIR"
  step "usage: che-install.sh" "$shell" "$SCRIPT" --help
  [ "$("$CHE_INSTALL_DIR/che" --version | awk '{print $3}')" = "$OLD" ]
  if "$shell" "$SCRIPT" --bogus 2>/dev/null; then
    echo "FAIL: unknown argument accepted" >&2
    exit 1
  fi
done
echo "che-install.sh: all scenarios passed"
##[<] 🤖🤖
