#!/bin/bash
# Replace the oclaude-* launchers on your PATH with symlinks to configs/*.sh, so the
# benchmark configs and your daily launchers are the same files (single source of truth).
#
#   ./install-launchers.sh            # auto-detects the dir of your current oclaude-high, else ~/bin
#   ./install-launchers.sh ~/.local/bin
#
# Existing launchers are kept as <name>.bak. Re-run any time; symlinks are refreshed.
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
mkdir -p "$TARGET"
for name in high normal normal-max light light-force; do
  src="$HERE/configs/$name.sh"
  dst="$TARGET/oclaude-$name"
  if [ -e "$dst" ] && [ ! -L "$dst" ]; then
    mv -n "$dst" "$dst.bak" && echo "backed up $dst -> $dst.bak"
  fi
  ln -sfn "$src" "$dst"
  chmod +x "$src"
  echo "oclaude-$name -> $src"
done
case ":$PATH:" in *":$TARGET:"*) ;; *) echo "note: $TARGET is not on your PATH";; esac
