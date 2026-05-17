#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="gopherhole"
DEFAULT_INSTALL_DIR="$HOME/.local/bin"
USER_SET_INSTALL_DIR=false
if [[ -v INSTALL_DIR ]]; then
  USER_SET_INSTALL_DIR=true
fi
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

check_exec_dir() {
  local dir="$1"
  local probe="${dir}/.exec-probe-$$.sh"
  mkdir -p "$dir"

  cat >"$probe" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
  chmod +x "$probe"

  set +e
  "$probe" >/dev/null 2>&1
  local rc=$?
  set -e

  rm -f "$probe"
  return "$rc"
}

if ! command -v go >/dev/null 2>&1; then
  echo "Error: Go is not installed or not in PATH." >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CANDIDATES=(
  "${XDG_BIN_HOME:-}"
  "$HOME/bin"
  "$HOME/.local/bin"
  "/tmp/$USER-bin"
  "$SCRIPT_DIR/build"
)

binary_runs() {
  local path="$1"
  set +e
  "$path" -h >/dev/null 2>&1
  local rc=$?
  set -e
  [[ "$rc" -eq 0 ]]
}

echo "Building ${BINARY_NAME}..."
go build -o "./build/${BINARY_NAME}" .

install_and_test() {
  local dir="$1"
  mkdir -p "$dir"
  cp "./build/${BINARY_NAME}" "${dir}/${BINARY_NAME}"
  chmod +x "${dir}/${BINARY_NAME}"
  binary_runs "${dir}/${BINARY_NAME}"
}

if [[ "$USER_SET_INSTALL_DIR" == "true" ]]; then
  if ! install_and_test "$INSTALL_DIR"; then
    echo "Error: installed binary cannot run from INSTALL_DIR: $INSTALL_DIR" >&2
    echo "Try one of these:" >&2
    echo "  INSTALL_DIR=\"$HOME/.local/bin\" ./install.sh" >&2
    echo "  INSTALL_DIR=\"$HOME/bin\" ./install.sh" >&2
    echo "  INSTALL_DIR=\"/tmp/$USER-bin\" ./install.sh" >&2
    exit 1
  fi
else
  if ! install_and_test "$INSTALL_DIR"; then
    echo "Warning: binary cannot run from $INSTALL_DIR. Searching fallback..."
    FOUND=""
    for candidate in "${CANDIDATES[@]}"; do
      [[ -z "$candidate" ]] && continue
      [[ "$candidate" == "$INSTALL_DIR" ]] && continue
      if install_and_test "$candidate"; then
        FOUND="$candidate"
        break
      fi
    done

    if [[ -z "$FOUND" ]]; then
      echo "Error: could not find any install directory where the binary runs." >&2
      echo "Tried:" >&2
      echo "  - $INSTALL_DIR" >&2
      for candidate in "${CANDIDATES[@]}"; do
        [[ -n "$candidate" ]] && [[ "$candidate" != "$INSTALL_DIR" ]] && echo "  - $candidate" >&2
      done
      exit 1
    fi

    INSTALL_DIR="$FOUND"
  fi
fi

echo "Installed to ${INSTALL_DIR}/${BINARY_NAME}"

case ":$PATH:" in
  *":${INSTALL_DIR}:"*)
    echo "${INSTALL_DIR} is already in PATH."
    ;;
  *)
    echo "\nAdd this to your shell profile (~/.zshrc, ~/.bashrc, etc.):"
    echo "export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

echo "\nNext steps:"
echo "  ${BINARY_NAME} init"
echo "  ${BINARY_NAME}"
