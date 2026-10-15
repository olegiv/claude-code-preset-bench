#!/bin/bash
# Replace the oclaude-* launchers on your PATH with symlinks to configs/*.sh, so the
# benchmark configs and your daily launchers are the same files (single source of truth).
#
#   ./install-launchers.sh            # auto-detects the dir of your current oclaude-high, else ~/bin
#   ./install-launchers.sh ~/.local/bin
#
# Existing launchers are kept as <name>.bak. If such a .bak already exists, nothing is
# changed: move it away first. Re-run any time; symlinks are refreshed.
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
TARGET="${1:-}"
if [ -z "$TARGET" ]; then
  if existing="$(command -v oclaude-high 2>/dev/null)"; then
    TARGET="$(dirname "$existing")"
  else
    TARGET="$HOME/bin"
  fi
fi
NAMES="high normal normal-max light light-force"
# Check every launcher first, so a refusal leaves all of them as they were.
conflict=0
for name in $NAMES; do
  dst="$TARGET/oclaude-$name"
  if [ -e "$dst" ] && [ ! -L "$dst" ] && { [ -e "$dst.bak" ] || [ -L "$dst.bak" ]; }; then
    echo "error: $dst.bak already exists, so $dst cannot be backed up; move one of them away first" >&2
    conflict=1
  fi
done
[ "$conflict" -eq 0 ] || exit 1
mkdir -p "$TARGET"
for name in $NAMES; do
  src="$HERE/configs/$name.sh"
  dst="$TARGET/oclaude-$name"
  if [ -e "$dst" ] && [ ! -L "$dst" ]; then
    mv "$dst" "$dst.bak"
    echo "backed up $dst -> $dst.bak"
  fi
  ln -sfn "$src" "$dst"
  chmod +x "$src"
  echo "oclaude-$name -> $src"
done
case ":$PATH:" in *":$TARGET:"*) ;; *) echo "note: $TARGET is not on your PATH";; esac
